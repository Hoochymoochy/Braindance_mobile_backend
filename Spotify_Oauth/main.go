package main
import "models"

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func loginHandler(w http.ResponseWriter, r *http.Request) {

	scope := "user-read-email user-top-read"

	authURL := fmt.Sprintf(
		"https://accounts.spotify.com/authorize?client_id=%s&response_type=code&redirect_uri=%s&scope=%s",
		os.Getenv("SPOTIFY_CLIENT_ID"),
		url.QueryEscape(os.Getenv("REDIRECT_URI")),
		url.QueryEscape(scope),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {

	code := r.URL.Query().Get("code")

	if code == "" {
		http.Error(w, "No authorization code received", http.StatusBadRequest)
		return
	}

	token, err := exchangeCodeForToken(code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user, err := getSpotifyUser(token.AccessToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":          user,
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
	})
}

func exchangeCodeForToken(code string) (*models.TokenResponse, error) {

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", os.Getenv("REDIRECT_URI"))

	req, err := http.NewRequest(
		"POST",
		"https://accounts.spotify.com/api/token",
		strings.NewReader(data.Encode()),
	)

	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(
		os.Getenv("SPOTIFY_CLIENT_ID"),
		os.Getenv("SPOTIFY_CLIENT_SECRET"),
	)

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var token models.TokenResponse

	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

func getSpotifyUser(accessToken string) (*models.SpotifyUser, error) {

	req, err := http.NewRequest(
		"GET",
		"https://api.spotify.com/v1/me",
		nil,
	)

	if err != nil {
		return nil, err
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+accessToken,
	)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var user models.SpotifyUser

	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func homeHandler(w http.ResponseWriter, r *http.Request) {

	fmt.Fprint(w, `
	<h1>Spotify OAuth Test</h1>
	<a href="/login">
		<button>Login with Spotify</button>
	</a>
	`)
}

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Could not load .env file")
	}

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/callback", callbackHandler)

	fmt.Println("Server running on http://localhost:8080")

	log.Fatal(http.ListenAndServe(":8080", nil))
}