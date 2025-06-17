NAME = "spotify-vc"

build:
	go build -o bin/$(NAME) 

run: build
	./bin/$(NAME)
