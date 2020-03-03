package main
import (
	"os"
	"io"
	"io/ioutil"
	"fmt"
	"net/http"
	"encoding/json"
	"time"
	"path/filepath"
	"log"
)

// Structs for Github release messages

type Asset struct {
	Name string `json:"name"`
	ContentType string `json:"content_type"`
	BrowserDownloadUrl string `json:"browser_download_url"`
	Size int64 `json:"size"`
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
		log.Fatalf("Error getting Mame releases: %v\n", err)
	}
	defer resp.Body.Close()

	// Read the body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading Mame releases response: %v\n", err)
	}

	// Unmarshall json
	releases := make([]Release,0)
	json.Unmarshal(body, &releases)
	if err != nil {
		log.Fatalf("Error unmarshalling json: %v\n", err)
	}

	// Get the latest release
	if len(releases) == 0 {
		log.Println(string(body))
		log.Fatal("No releases found on Github")
	}
	latestRelease := releases[0]
	for _, release := range releases {

		if release.PublishedAt.After(latestRelease.PublishedAt) {
			latestRelease = release
		}
    }

	return latestRelease
}

func download(release Release) Asset {
	client := &http.Client{}

	for _, asset := range release.Assets {
		// We're interested in .exe files, 
		// excluding the self-extracting source archive
		// ie mame0219b_64bit.exe, but not mame0219s.exe
		filename := asset.Name
		extension := filepath.Ext(filename)
		name := filename[0:len(filename)-len(extension)]
		if len(name) < 4 || extension != ".exe" || name[len(name)-1:] == "s" {
			fmt.Printf("Skipping %s\n", asset.Name)
			continue
		} else {
			info, _ := os.Stat(filename)
			if info != nil && info.Size() == asset.Size {
				fmt.Printf("Release %s already downloaded.\n", asset.Name)
				continue
			}
		}
		fmt.Printf("Downloading %s (%dM)...\n", asset.Name, asset.Size / (1024*1024))

		// Call the API
		resp, err := client.Get(asset.BrowserDownloadUrl)
		if err != nil {
			log.Fatalf("Error getting Mame release %s: %v\n", asset.Name, err)
		}
		defer resp.Body.Close()

		// Create the file
		out, err := os.Create(filename)
		if err != nil {
			log.Fatalf("Error creating file %s\n", filename)
		}
		defer out.Close()
	
		// Download the body
		written, err := io.Copy(out, resp.Body)
		if err != nil {
			log.Fatalf("Error writing output file: %v\n", err)
		} else if written != asset.Size {
			log.Fatalf("Expected to download %d bytes, but actually got %d bytes.", asset.Size, written)
		}

		return asset
	}

	return Asset{}
}

func save(name string, size int64) {

	from, err := os.Open(name)
	if err != nil {
		log.Fatalf("Error opening input file %s: %v\n", name, err)
	}
	defer from.Close()
	
	to, err := os.OpenFile("mame.exe", os.O_RDWR|os.O_CREATE, 0775)
	if err != nil {
		log.Fatalf("Error opening output file %s: %v\n", "mame.exe", err)
	}
	defer to.Close()
	
	_, err = io.Copy(to, from)
	if err != nil {
		log.Fatalf("Error copying file: %v", err)
	}
}

func main() {
	fmt.Printf("Checking releases\n")
	release := getLatestRelease()
	fmt.Printf("Latest release is %s, published at %v\n", release.Name, release.PublishedAt)
	asset := download(release)
	if asset.Name != "" {
		fmt.Printf("Looks like a successful download. Saving.\n")
		// TODO check size / SHA
		save(name, size)
	}
	fmt.Println("Fin.")
}