package androidbuilder

import (
	"archive/zip"
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func GetPakageFromManifest(manifestPath string) (string, error) {
	var manifest struct {
		XMLName xml.Name `xml:"manifest"`
		Package string   `xml:"package,attr"`
	}

	f, err := os.Open(manifestPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	err = xml.NewDecoder(f).Decode(&manifest)
	if err != nil {
		return "", err
	}

	return manifest.Package, nil
}

func GetPackageAndActivityFromManifest(manifestFile string) (pkgName string, activityName string, err error) {
	var manifest struct {
		XMLName     xml.Name `xml:"manifest"`
		Package     string   `xml:"package,attr"`
		Application struct {
			Activity []struct {
				Name         string `xml:"name,attr"`
				IntentFilter []struct {
					Action struct {
						Name string `xml:"name,attr"`
					} `xml:"action"`
				} `xml:"intent-filter"`
			} `xml:"activity"`
		} `xml:"application"`
	}

	f, err := os.Open(manifestFile)
	if err != nil {
		return "", "", fmt.Errorf("findPackageNameAndActivityName: %w", err)
	}
	defer f.Close()

	d := xml.NewDecoder(f)
	err = d.Decode(&manifest)
	if err != nil {
		return "", "", fmt.Errorf("findPackageNameAndActivityName: %w", err)
	}

	for _, activity := range manifest.Application.Activity {
		for _, intentFilter := range activity.IntentFilter {
			if intentFilter.Action.Name == "android.intent.action.MAIN" {
				return manifest.Package, activity.Name, nil
			}
		}
	}

	return "", "", errors.New("unable to find")
}

func FindMinSdkAndTargetSdk(androidDir string) (string, string, error) {
	// Try parsing AndroidManifest.xml
	{
		manifestFile := filepath.Join(androidDir, "app", "src", "main", "AndroidManifest.xml")

		var manifest struct {
			XMLName xml.Name `xml:"manifest"`
			UsesSdk struct {
				MinSdkVersion    string `xml:"minSdkVersion,attr"`
				TargetSdkVersion string `xml:"targetSdkVersion,attr"`
			} `xml:"uses-sdk"`
		}

		f, err := os.Open(manifestFile)
		if err != nil {
			return "", "", err
		}
		defer f.Close()

		err = xml.NewDecoder(f).Decode(&manifest)
		if err != nil {
			return "", "", err
		}

		if manifest.UsesSdk.MinSdkVersion != "" && manifest.UsesSdk.TargetSdkVersion != "" {
			return manifest.UsesSdk.MinSdkVersion, manifest.UsesSdk.TargetSdkVersion, nil
		}
	}

	// Try parsing from build.gradle
	{
		buildGradle := filepath.Join(androidDir, "app", "build.gradle")

		f, err := os.Open(buildGradle)
		if err != nil {
			return "", "", err
		}
		defer f.Close()

		var minSdk, targetSdk string

		s := bufio.NewScanner(f)
		for s.Scan() {
			text := strings.TrimSpace(s.Text())

			if strings.HasPrefix(text, "minSdk") {
				split := strings.Split(text, " ")
				if len(split) > 1 {
					minSdk = split[1]
				}
			}

			if strings.HasPrefix(text, "targetSdk") {
				split := strings.Split(text, " ")
				if len(split) > 1 {
					targetSdk = split[1]
				}
			}
		}

		if minSdk != "" && targetSdk != "" {
			return minSdk, targetSdk, nil
		}
	}

	return "", "", errors.New("unable to find minSdk and targetSdk")
}

func addFilesToZip(zipPath string, files map[string]string) error {
	err := os.Rename(zipPath, zipPath+".old")
	if err != nil {
		return err
	}

	z, err := zip.OpenReader(zipPath + ".old")
	if err != nil {
		return err
	}
	defer z.Close()

	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	for _, file := range z.File {
		err = func(file *zip.File, w *zip.Writer) error {
			src, err := file.Open()
			if err != nil {
				return err
			}
			defer src.Close()

			fh, err := zip.FileInfoHeader(file.FileInfo())
			if err != nil {
				return err
			}
			fh.Name = file.Name

			dst, err := w.CreateHeader(fh)
			if err != nil {
				return err
			}

			_, err = io.Copy(dst, src)
			if err != nil {
				return err
			}

			return nil
		}(file, w)
		if err != nil {
			return err
		}
	}

	for pathOnHost, pathInZip := range files {
		err = func() error {
			src, err := os.Open(pathOnHost)
			if err != nil {
				return err
			}
			defer src.Close()

			info, err := src.Stat()
			if err != nil {
				return err
			}

			fh, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			fh.Name = pathInZip

			dst, err := w.CreateHeader(fh)
			if err != nil {
				return err
			}

			_, err = io.Copy(dst, src)
			if err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

// only supports "build-tools" & "ndk"
func FindLatestVersionOfSdk(sdk string, targetSdkVersion string, skipPreview bool) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://dl.google.com/android/repository/repository2-1.xml", nil)
	if err != nil {
		return "", fmt.Errorf("findLatestVersionOfSdk: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("findLatestVersionOfSdk: %w", err)
	}
	defer res.Body.Close()

	var repo struct {
		XMLName       xml.Name `xml:"sdk-repository"`
		RemotePackage []struct {
			Path     string `xml:"path,attr"`
			Revision struct {
				Major   string `xml:"major"`
				Minor   string `xml:"minor"`
				Micro   string `xml:"micro"`
				Preview string `xml:"preview"`
			} `xml:"revision"`
		} `xml:"remotePackage"`
	}

	d := xml.NewDecoder(res.Body)
	err = d.Decode(&repo)
	if err != nil {
		return "", fmt.Errorf("findLatestVersionOfSdk: %w", err)
	}

	for _, pkg := range repo.RemotePackage {
		// skip release candidates or beta releases
		if skipPreview && pkg.Revision.Preview != "" {
			continue
		}

		switch sdk {
		case "build-tools":
			if strings.HasPrefix(pkg.Path, "build-tools") && pkg.Revision.Major == targetSdkVersion {
				// assume first hit is latest one
				// since xml should already be in sorted form
				return pkg.Revision.Major + "." + pkg.Revision.Minor + "." + pkg.Revision.Micro, nil
			}

		case "ndk":
			if strings.HasPrefix(pkg.Path, "ndk") {
				// assume first hit is latest one
				// since xml should already be in sorted form
				return pkg.Revision.Major + "." + pkg.Revision.Minor + "." + pkg.Revision.Micro, nil
			}
		}
	}

	return "", errors.New("findLatestVersionOfSdk: unable to find latest version for " + sdk)
}
