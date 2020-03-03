package main
import (
	"os"
	"io"
	"io/ioutil"
	"net/http"
	"encoding/json"
	"time"
	"path/filepath"
	"log"
	"bufio"
	"strings"
	"crypto/sha256"
	"encoding/hex"
	"archive/zip"
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

func downloadAsset(asset Asset, client *http.Client) {

	filename := asset.Name

	info, _ := os.Stat(asset.Name)
	if info != nil && info.Size() == asset.Size {
		log.Printf("Asset %s already downloaded.\n", asset.Name)
		return
	}
	log.Printf("Downloading %s (%dM)...\n", asset.Name, asset.Size / (1024*1024))

	// Open the URL
	resp, err := client.Get(asset.BrowserDownloadUrl)
	if err != nil {
		log.Fatalf("Error getting %s: %v\n", asset.Name, err)
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Error creating download file %s\n", filename)
	}
	defer out.Close()

	// Download the body
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		log.Fatalf("Error writing download file: %v\n", err)
	} else if written != asset.Size {
		log.Fatalf("Expected to download %d bytes, but actually got %d bytes.", asset.Size, written)
	}
}

func download(release Release) (mame, xml, sha256sums Asset) {
	client := &http.Client{}

	for _, asset := range release.Assets {
		// We're interested in .exe files, 
		// excluding the self-extracting source archive
		// ie mame0219b_64bit.exe, but not mame0219s.exe
		filename := asset.Name
		extension := filepath.Ext(filename)
		name := filename[0:len(filename)-len(extension)]

		if extension == ".exe" && len(name) > 5 && name[len(name)-5:] == "64bit" {
			log.Printf("Mame: %s\n", asset.Name)
			mame = asset
			downloadAsset(mame, client)
		}

		if extension == ".zip" && len(name) > 2 && name[len(name)-2:] == "lx" {
			log.Printf("XML: %s\n", asset.Name)
			xml = asset
			downloadAsset(xml, client)
		}

		if filename == "SHA256SUMS" {
			log.Printf("SHA256 checksums: %s\n", asset.Name)
			sha256sums = asset
			downloadAsset(sha256sums, client)
		}
	}

	return
}

func checksum(asset Asset, checksums map[string]string) {

	info, _ := os.Stat(asset.Name)
	if info != nil && info.Size() != asset.Size {
		log.Fatalf("Asset %s expected to be %d bytes, but is %d bytes.\n", asset.Name, asset.Size, info.Size())
	}

	expected := checksums[asset.Name]

	f, err := os.Open(asset.Name)
	if err != nil {
		log.Fatalf("Error opening %s: %v", asset.Name, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatalf("Error checksumming %s: %v", asset.Name, err)
	}

	actual := hex.EncodeToString(h.Sum(nil))

	if expected != actual {
		log.Fatalf("SHA256 mismatch for %s. Expected %s but got %s\n", asset.Name, expected, actual)
	}

	log.Printf("SHA256 matches: %s\n", asset.Name)
}

func readChecksums(sha256sums Asset) (checksums map[string]string) {
	checksums = make(map[string]string)

	file, err := os.Open(sha256sums.Name)
    if err != nil {
        log.Fatalf("Error opening checksums file: %v", err)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
		segments := strings.Split(scanner.Text(), " ")
		checksum := segments[0]
		name := strings.TrimPrefix(segments[1], "*")
		checksums[name] = checksum
    }

    if err := scanner.Err(); err != nil {
        log.Fatalf("Error reading checksums: %v", err)
    }

	return
}

func extract(zipFile Asset) (extracted string) {

	// Open the zip archive for reading.
	r, err := zip.OpenReader(zipFile.Name)
	if err != nil {
		log.Fatalf("Error opening zip file %s: %v", zipFile.Name, err)
	}
	defer r.Close()

	// Iterate through and extract contents.
	for _, f := range r.File {
		filename := f.Name
		extension := filepath.Ext(filename)
		if extension == ".xml" {
			extracted = filename

			from, err := f.Open()
			if err != nil {
				log.Fatalf("Error opening zip entry %s: %v\n", f.Name, err)
			}
			defer from.Close()
			
			to, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0664)
			if err != nil {
				log.Fatalf("Error opening output file %s: %v\n", filename, err)
			}
			defer to.Close()
			
			_, err = io.Copy(to, from)
			if err != nil {
				log.Fatalf("Error extracting file: %v", err)
			}
		}
	}

	return
}

func save(source, destination string) {

	extension := filepath.Ext(source)
	var permissions os.FileMode = 0664
	if extension == ".exe" {
		permissions = 0775
	}

	from, err := os.Open(source)
	if err != nil {
		log.Fatalf("Error opening input file %s: %v\n", source, err)
	}
	defer from.Close()
	
	to, err := os.OpenFile(destination, os.O_RDWR|os.O_CREATE, permissions)
	if err != nil {
		log.Fatalf("Error opening output file %s: %v\n", destination, err)
	}
	defer to.Close()
	
	_, err = io.Copy(to, from)
	if err != nil {
		log.Fatalf("Error copying file: %v", err)
	}
}

func main() {

	// Find the latest release
	log.Printf("Checking releases\n")
	release := getLatestRelease()
	log.Printf("Latest release is %s, published at %v\n", release.Name, release.PublishedAt)

	// Download release assets
	mame, xmlZip, sha256sums := download(release)
	if mame.Name == "" || xmlZip.Name == "" || sha256sums.Name == "" {
		log.Fatalf("Didn't find all the files we're looking for. Mame: %s, XML: %s, checksum: %s", mame.Name, xmlZip.Name, sha256sums.Name)
	}

	// Verify integrity
	checksums := readChecksums(sha256sums)
	checksum(mame, checksums)
	checksum(xmlZip, checksums)

	// Save the results
	log.Printf("Looks like a successful download. Saving.\n")
	xml := extract(xmlZip)
	if xml == "" {
		log.Fatalf("No xml extracted from %s\n", xmlZip.Name)
	} else {
		log.Printf("Extracted xml: %s\n", xml)
	}
	save(mame.Name, "mame.exe")
	save(xml, "mame.xml")

	log.Println("Fin.")
}