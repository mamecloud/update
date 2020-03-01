package main
import (
	"os"
	"io/ioutil"
	"fmt"
	"net/http"
	"encoding/json"
	"time"
	"path/filepath"
)

type Asset struct {
	Name string `json:"name"`
	ContentType string `json:"content_type"`
	BrowserDownloadUrl string `json:"browser_download_url"`
}

type Release struct {
	HtmlUrl string `json:"html_url"`
	TagName string `json:"tag_name"`
	Name string `json:"name"`
	Draft bool `json:"draft"`
	PreRelease bool `json:"prerelease"`
	CreatedAt time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at"`
	Assets []Asset `json:"assets"`
}

func getLatestRelease() Release {
	client := &http.Client{}

	// Call the API
	resp, err := client.Get("https://api.github.com/repos/mamedev/mame/releases")
	if err != nil {
		fmt.Printf("Error getting Mame releases: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Read the body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading Mame releases response: %v\n", err)
		os.Exit(1)
	}

	// Unmarshall json
	releases := make([]Release,0)
	json.Unmarshal(body, &releases)
	if err != nil {
		fmt.Printf("Error unmarshalling json: %v\n", err)
		os.Exit(1)
	}

	// Get the latest release
	if len(releases) == 0 {
		fmt.Printf("No releases found on Github")
		os.Exit(1)
	}
	latestRelease := releases[0]
	for _, release := range releases {

		if release.PublishedAt.After(latestRelease.PublishedAt) {
			latestRelease = release
		}
    }

	return latestRelease
}

func download(release Release) {
	client := &http.Client{}

	for _, asset := range release.Assets {
		name := asset.Name
		fmt.Println("Asset: %s", name)
		if len(name) < 4 || strings.ToLower(filepath.Ext(name)) != ".exe" {
			continue
		}
		fmt.Printf("Downloading %s from %s", name, asset.BrowserDownloadUrl)

		// Call the API
		resp, err := client.Get(asset.BrowserDownloadUrl)
		if err != nil {
			fmt.Printf("Error getting Mame release %s: %v\n", asset.Name, err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		// Create the file
		out, err := os.Create(name)
		if err != nil {
			fmt.Printf("Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer out.Close()
	
		// Download the body
		_, err := io.Copy(out, resp.Body)
		if err != nil {
			fmt.Printf("Error writing output file: %v\n", err)
			os.Exit(1)
		}
	}
}

func main() {
	release := getLatestRelease()
	download(release)
	fmt.Printf("Latest release is %s, published at %v\n", release.Name, release.PublishedAt)
	fmt.Println("Fin.")
}