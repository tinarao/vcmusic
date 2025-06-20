package spotify

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type authResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type Track struct {
	Name     string `json:"name"`
	Duration int    `json:"duration_ms"`
	Album    struct {
		Name   string `json:"name"`
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
	} `json:"album"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
}

type SearchResponse struct {
	Tracks struct {
		Items []Track `json:"items"`
	} `json:"tracks"`
}

func Search() (SearchResponse, error) {
	query := "freddie%20gibbs"
	rawUrl := fmt.Sprintf("https://api.spotify.com/v1/search?q=%s&limit=1&type=album,track", query)

	var searchResp SearchResponse

	req, err := http.NewRequest("GET", rawUrl, nil)
	if err != nil {
		return searchResp, err
	}

	token, err := getAccessToken()
	if err != nil {
		return searchResp, errors.New("unauthorized")
	}

	req.Header.Set("Authorization", "Bearer "+token)
	client := http.Client{
		Timeout: time.Second * 2,
	}

	resp, err := client.Do(req)
	if err != nil {
		return searchResp, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return searchResp, err
	}

	err = json.Unmarshal(body, &searchResp)
	if err != nil {
		return searchResp, err
	}

	fmt.Printf("%+v\n", searchResp)

	return searchResp, nil
}

func getAccessToken() (string, error) {
	authURL := "https://accounts.spotify.com/api/token"

	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", authURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	clientId := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientId == "" || clientSecret == "" {
		panic("fool")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientId, clientSecret)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var authResp authResponse
	err = json.Unmarshal(body, &authResp)
	if err != nil {
		return "", err
	}

	return authResp.AccessToken, nil
}
