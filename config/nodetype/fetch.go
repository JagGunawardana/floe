package nodetype

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cavaliercoder/grab"
)

type fetchOpts struct {
	URL          string `json:"url"`           // url of the file to download
	Checksum     string `json:"checksum"`      // the checksum (and typically filename)
	ChecksumAlgo string `json:"checksum-algo"` // the checksum algorithm
	Location     string `json:"location"`      // where to download
}

// fetch downloads stuff if it is not in the cache
type fetch struct{}

func (g fetch) Match(ol, or Opts) bool {
	return true
}

func (g fetch) Execute(ws *Workspace, in Opts, output chan string) (int, Opts, error) {

	fop := fetchOpts{}
	err := decode(in, &fop)
	if err != nil {
		return 255, nil, err
	}

	if fop.URL == "" {
		return 255, nil, fmt.Errorf("problem getting fetch url option")
	}
	if fop.Checksum == "" {
		output <- "(N.B. fetch without a checksum can not be trusted)"
	}

	client := grab.NewClient()
	req, err := grab.NewRequest(ws.FetchCache, fop.URL)
	if err != nil {
		output <- fmt.Sprintf("Error setting up the download %v", err)
		return 255, nil, err
	}

	// set up any checksum
	if len(fop.Checksum) > 0 {
		// is it in the sum filename format e.g. ba411cafee2f0f702572369da0b765e2  bodhi-4.1.0-64.iso
		parts := strings.Split(fop.Checksum, " ")
		if len(parts) > 1 {
			fop.Checksum = parts[0]
		}
		checksum, err := hex.DecodeString(fop.Checksum)
		if err != nil {
			output <- fmt.Sprintf("Error decoding hex checksum: %s", fop.Checksum)
			return 255, nil, err
		}

		var h hash.Hash
		switch fop.ChecksumAlgo {
		case "sha256":
			h = sha256.New()
		case "sha1":
			h = sha1.New()
		case "md5":
			h = md5.New()
		}
		req.SetChecksum(h, checksum, true)
	}

	started := time.Now()
	// start download
	output <- fmt.Sprintf("Downloading %v...", req.URL())
	resp := client.Do(req)
	output <- fmt.Sprintf("  %v", resp.HTTPResponse.Status)

	// start UI loop
	t := time.NewTicker(300 * time.Millisecond)
	defer t.Stop()

	// start reporting updates
Loop:
	for {
		select {
		case <-t.C:
			output <- fmt.Sprintf("  %v / %v bytes (%.2f%%)", resp.BytesComplete(), resp.Size, 100*resp.Progress())
		case <-resp.Done:
			break Loop
		}
	}
	// check for errors, emit it and bail
	if err := resp.Err(); err != nil {
		output <- fmt.Sprintf("Download failed: %v", err)
		return 255, nil, nil // don't consider this an err, just that it failed
	}
	output <- fmt.Sprintf("  %v / %v bytes (%.2f%%) in %v", resp.BytesComplete(), resp.Size, 100*resp.Progress(), time.Since(started))
	output <- fmt.Sprintf("Download saved to %v", resp.Filename)

	// if no location was given to link it to then link it to the root of the workspace
	// this will be used to link to the file in the cache

	println(filepath.Base(resp.Filename))
	toLink := linkLocation(fop.Location, filepath.Base(resp.Filename))
	toLink = expandEnv(toLink, ws.BasePath)
	println(toLink)

	// make sure it is truly not there
	// but dont care if there is nothing to remove later failures
	// will result if this fails when it should have succeeded
	os.Remove(toLink)

	// make sure the folder exists for the link
	err = os.MkdirAll(filepath.Dir(toLink), 0700)
	if err != nil {
		return 255, nil, err
	}

	err = os.Link(resp.Filename, toLink)
	if err != nil {
		return 255, nil, err
	}
	output <- fmt.Sprintf("Download linked to %v", toLink)

	return 0, nil, nil
}

func linkLocation(loc, name string) string {
	// no location given so use download filename in root workspace
	if loc == "" {
		return filepath.Join(wsSub, name)
	}
	// if the location is not a file name (ends in '/')
	// then the location is the location part plus the name
	if loc[len(loc)-1] == filepath.Separator {
		loc = filepath.Join(loc, name)
	}
	// relative paths are relative to the workspace so prepend with the
	// substitution string for the workspace
	if loc[0] != filepath.Separator {
		loc = filepath.Join(wsSub, loc)
	}
	return loc
}
