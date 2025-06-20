package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"encoding/json"

	vosk "github.com/alphacep/vosk-api/go"
	"github.com/eiannone/keyboard"
	"github.com/gordonklaus/portaudio"
)

type partialObj struct {
	Partial string `json:"partial"`
}

type resultObj struct {
	Text string `json:"text"`
}

const (
	modelPath       = "model"
	sampleRate      = 16000
	framesPerBuffer = 1024
	inputChannels   = 1
	outputChannels  = 0

	silenceThreshold = 0.01
	silenceDuration  = 500 * time.Millisecond
)

type Config struct {
	ModelPath       string
	SampleRate      float64
	FramesPerBuffer int
	InputChannels   int
	OutputChannels  int
}

type AudioBuffer struct {
	data     []float32
	lastTime time.Time
}

func NewConfig() *Config {
	return &Config{
		ModelPath:       modelPath,
		SampleRate:      float64(sampleRate),
		FramesPerBuffer: framesPerBuffer,
		InputChannels:   inputChannels,
		OutputChannels:  outputChannels,
	}
}

func NewAudioBuffer() *AudioBuffer {
	return &AudioBuffer{
		data:     make([]float32, 0),
		lastTime: time.Now(),
	}
}

func (ab *AudioBuffer) Add(samples []float32) {
	ab.data = append(ab.data, samples...)
	ab.lastTime = time.Now()
}

func (ab *AudioBuffer) Clear() {
	ab.data = make([]float32, 0)
}

func (ab *AudioBuffer) IsSilence() bool {
	return time.Since(ab.lastTime) > silenceDuration
}

func (ab *AudioBuffer) HasContent() bool {
	return len(ab.data) > 0
}

func main() {
	config := NewConfig()

	if err := validateModelPath(config.ModelPath); err != nil {
		log.Fatalf("Ошибка валидации модели: %v", err)
	}

	model, err := initVoskModel(config.ModelPath)
	if err != nil {
		log.Fatalf("Ошибка инициализации модели Vosk: %v", err)
	}
	defer func() {
		model.Free()
	}()

	recognizer, err := initRecognizer(model)
	if err != nil {
		log.Fatalf("Ошибка инициализации распознавателя: %v", err)
	}

	stream, err := initAudioStream(config, recognizer)
	if err != nil {
		log.Fatalf("Ошибка инициализации аудиопотока: %v", err)
	}
	defer func() {
		if err := stream.Close(); err != nil {
			log.Printf("Ошибка при закрытии потока: %v", err)
		}
	}()

	if err := stream.Start(); err != nil {
		log.Fatalf("Ошибка запуска аудиопотока: %v", err)
	}
	defer func() {
		if err := stream.Stop(); err != nil {
			log.Printf("Ошибка при остановке потока: %v", err)
		}
	}()

	if err := keyboard.Open(); err != nil {
		log.Fatalf("Ошибка инициализации клавиатуры: %v", err)
	}
	defer keyboard.Close()

	fmt.Println("Нажмите R для начала/окончания записи. Ctrl+C для выхода.")

	done := make(chan struct{})

	go func() {
		for {
			char, key, err := keyboard.GetKey()
			if err != nil {
				log.Printf("Ошибка чтения клавиши: %v", err)
				continue
			}

			if key == keyboard.KeyCtrlC {
				close(done)
				return
			}

			if char == 'r' || char == 'R' {
				recordingChan <- struct{}{}
			}
		}
	}()

	<-done
	fmt.Println("\nЗавершение...")
}

var recordingChan = make(chan struct{})

func validateModelPath(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("директория модели Vosk не существует")
	}
	return nil
}

func initVoskModel(path string) (*vosk.VoskModel, error) {
	model, err := vosk.NewModel(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать модель: %w", err)
	}
	return model, nil
}

func initRecognizer(model *vosk.VoskModel) (*vosk.VoskRecognizer, error) {
	if err := portaudio.Initialize(); err != nil {
		return nil, fmt.Errorf("ошибка инициализации PortAudio: %w", err)
	}

	recognizer, err := vosk.NewRecognizer(model, float64(sampleRate))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания распознавателя: %w", err)
	}
	return recognizer, nil
}

func initAudioStream(config *Config, recognizer *vosk.VoskRecognizer) (*portaudio.Stream, error) {
	stream, err := portaudio.OpenDefaultStream(
		config.InputChannels,
		config.OutputChannels,
		config.SampleRate,
		config.FramesPerBuffer,
		processAudio(recognizer),
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия аудиопотока: %w", err)
	}
	return stream, nil
}

func processAudio(recognizer *vosk.VoskRecognizer) func([]float32) {
	buffer := NewAudioBuffer()
	isRecording := false

	go func() {
		for range recordingChan {
			if isRecording {
				isRecording = false
				if buffer.HasContent() {
					processPhrase(recognizer, buffer.data)
					buffer.Clear()
				}
				fmt.Println("Запись остановлена")
			} else {
				isRecording = true
				buffer.Clear()
				fmt.Println("Запись начата...")
			}
		}
	}()

	return func(samples []float32) {
		if recognizer == nil || !isRecording {
			return
		}
		buffer.Add(samples)
	}
}

func processPhrase(recognizer *vosk.VoskRecognizer, data []float32) {
	int16buff := make([]int16, len(data))
	for i, sample := range data {
		int16buff[i] = int16(sample * 32767.0)
	}

	bytesbuff := make([]byte, len(int16buff)*2)
	for i, sample := range int16buff {
		bytesbuff[i*2] = byte(sample)
		bytesbuff[i*2+1] = byte(sample >> 8)
	}

	if recognizer.AcceptWaveform(bytesbuff) == 0 {
		if result := recognizer.Result(); result != "" {
			var res resultObj
			if err := json.Unmarshal([]byte(result), &res); err == nil {
				fmt.Println(res.Text)
			} else {
				fmt.Println(result)
			}
		}
	} else {
		if partial := recognizer.PartialResult(); partial != "" {
			var part partialObj
			if err := json.Unmarshal([]byte(partial), &part); err == nil {
				fmt.Println(part.Partial)
			} else {
				fmt.Println(partial)
			}
		}
	}
}
