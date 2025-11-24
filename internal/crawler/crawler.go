package crawler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-crawler-client/config"
	"go-crawler-client/internal/model"
	"go-crawler-client/internal/service"

	"github.com/gocolly/colly/v2"
)

// GetUserInfo get the user info (sync)
func GetUserInfo(userID string, cookie string) (model.UserInfo, error) {
	// Check if proxy is configured
	client := &http.Client{}
	if config.GlobalConfig.ProxyHost != "" {
		proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%d", config.GlobalConfig.ProxyHost, config.GlobalConfig.ProxyPort))
		if err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		}
	}

	// Request Pixiv API
	apiURL := fmt.Sprintf("https://www.pixiv.net/ajax/user/%s", userID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return model.UserInfo{}, err
	}

	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return model.UserInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return model.UserInfo{}, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	var apiResp struct {
		Body struct {
			UserID   string `json:"userId"`
			Name     string `json:"name"`
			ImageBig string `json:"imageBig"`
		} `json:"body"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return model.UserInfo{}, err
	}

	// Download avatar
	baseDir := config.GetBaseDir()
	avatarDir := filepath.Join(baseDir, "crawl-datas", userID, ".avatars")
	if _, err := os.Stat(avatarDir); os.IsNotExist(err) {
		os.MkdirAll(avatarDir, 0755)
	}
	avatarPath := filepath.Join(avatarDir, userID+".jpg")
	// Download with Referer
	err = downloadFileWithReferer(apiResp.Body.ImageBig, avatarPath, "https://www.pixiv.net/")
	if err != nil {
		// Log error but do not interrupt the process
		fmt.Printf("Warning: failed to download avatar: %v\n", err)
	}

	return model.UserInfo{
		UserID:     apiResp.Body.UserID,
		Name:       apiResp.Body.Name,
		AvatarURL:  apiResp.Body.ImageBig,
		AvatarPath: avatarPath,
		Premium:    false,
	}, nil
}

// StartCrawler starts the crawling process for a given task
func StartCrawler(task *service.Task, cookie string) {
	// Crash recovery
	// If there is a bug in the crawler code causing a Panic, this defer will catch it to prevent the entire program from crashing
	// and mark the task status as failed
	defer func() {
		if r := recover(); r != nil {
			task.Logger.Error("Crawler panicked: %v", r)
			task.UpdateStatus("failed")
		}
	}()

	task.Logger.Info("Starting spider for user %s with mode %s", task.UserInfo.UserID, task.Mode)

	// Initialize Colly collector
	// colly.Async(true) enables asynchronous mode, allowing multiple requests to be sent in parallel
	c := colly.NewCollector(
		colly.Async(true),
	)

	// Set proxy (if configured)
	if config.GlobalConfig.ProxyHost != "" {
		proxyURL := fmt.Sprintf("http://%s:%d", config.GlobalConfig.ProxyHost, config.GlobalConfig.ProxyPort)
		c.SetProxy(proxyURL)
	}

	// Concurrency limit (very important!)
	// Limit to a maximum of 5 concurrent requests, with a random delay between each request to prevent Pixiv from banning the IP
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 5,
		RandomDelay: 1 * time.Second,
	})

	// Request callback: automatically add Cookie and User-Agent before each request
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Cookie", cookie)
		r.Headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	})

	// Error callback: log errors
	c.OnError(func(r *colly.Response, err error) {
		task.Logger.Error("Request URL: %s failed with response: %v\nError: %v", r.Request.URL, r, err)
	})

	// 1. Handle Profile All (Get Illust IDs)
	c.OnResponse(func(r *colly.Response) {
		if strings.Contains(r.Request.URL.String(), "/profile/all") {
			var resp struct {
				Body struct {
					Illusts map[string]any `json:"illusts"`
				} `json:"body"`
			}
			if err := json.Unmarshal(r.Body, &resp); err != nil {
				task.Logger.Error("Failed to parse profile: %v", err)
				return
			}

			task.Logger.Info("Found %d illusts", len(resp.Body.Illusts))

			for id := range resp.Body.Illusts {
				detailURL := fmt.Sprintf("https://www.pixiv.net/ajax/illust/%s", id)
				r.Request.Visit(detailURL)
			}
		}
	})

	// 2. Handle Illust Detail (Get Image URL)
	c.OnResponse(func(r *colly.Response) {
		if strings.Contains(r.Request.URL.String(), "/ajax/illust/") && !strings.Contains(r.Request.URL.String(), "profile") {
			var resp struct {
				Body struct {
					Id       string `json:"id"`
					Title    string `json:"title"`
					UserName string `json:"userName"`
					Urls     struct {
						Original string `json:"original"`
					} `json:"urls"`
				} `json:"body"`
			}
			if err := json.Unmarshal(r.Body, &resp); err != nil {
				return
			}

			if resp.Body.Urls.Original == "" {
				return
			}

			imgURL := resp.Body.Urls.Original
			task.Logger.Info("Found image: %s", imgURL)

			if task.Mode == "image" {
				baseDir := config.GetBaseDir()
				fileName := filepath.Base(imgURL)
				savePath := filepath.Join(baseDir, "crawl-datas", task.UserInfo.UserID, ".download_imgs", fileName)

				// Download with Referer
				err := downloadFileWithReferer(imgURL, savePath, "https://www.pixiv.net/")
				status := "success"
				if err != nil {
					status = "failed"
					task.Logger.Error("Failed to download image %s: %v", imgURL, err)
				} else {
					task.Logger.Info("Downloaded image to %s", savePath)
				}

				task.AddImage(model.ImageInfo{
					URL:      imgURL,
					Path:     savePath,
					Checksum: "",
					Status:   status,
				})
			}

			task.AddResult(model.TaskResult{
				UserID:    task.UserInfo.UserID,
				UserName:  resp.Body.UserName,
				ImageURLs: []string{imgURL},
			})
		}
	})

	// Start visiting
	profileURL := fmt.Sprintf("https://www.pixiv.net/ajax/user/%s/profile/all", task.UserInfo.UserID)
	c.Visit(profileURL)

	c.Wait()

	task.Logger.Info("Crawler finished")
	task.UpdateStatus("completed")

	saveTaskData(task)
}

func downloadFileWithReferer(url string, filepath string, referer string) error {
	client := &http.Client{}
	if config.GlobalConfig.ProxyHost != "" {
		proxyURL, err := urlParse(fmt.Sprintf("http://%s:%d", config.GlobalConfig.ProxyHost, config.GlobalConfig.ProxyPort))
		if err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Referer", referer)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status code: %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// Helper wrapper for url.Parse to avoid import conflict if any (though we imported net/url as url)
// Actually we imported "net/url", so we should use url.Parse
func urlParse(rawurl string) (*url.URL, error) {
	return url.Parse(rawurl)
}

// This function is responsible for writing the in-memory results to disk after the task is completed
func saveTaskData(task *service.Task) {
	baseDir := config.GetBaseDir()
	userDir := filepath.Join(baseDir, "crawl-datas", task.UserInfo.UserID)

	// Save task results
	resultFile := filepath.Join(userDir, ".task_data", fmt.Sprintf("task_%s.jsonl", task.ID))
	f, err := os.Create(resultFile)
	if err == nil {
		defer f.Close()
		encoder := json.NewEncoder(f)
		for _, res := range task.Results {
			encoder.Encode(res)
		}
	}

	// Save images info
	if len(task.Images) > 0 {
		imgFile := filepath.Join(userDir, ".task_data", fmt.Sprintf("task_%s_images.jsonl", task.ID))
		fImg, err := os.Create(imgFile)
		if err == nil {
			defer fImg.Close()
			encoder := json.NewEncoder(fImg)
			for _, img := range task.Images {
				encoder.Encode(img)
			}
		}
	}

	// Save final summary
	summaryFile := filepath.Join(userDir, ".task_results", fmt.Sprintf("task_%s_summary.json", task.ID))
	fSum, err := os.Create(summaryFile)
	if err == nil {
		defer fSum.Close()
		encoder := json.NewEncoder(fSum)
		encoder.SetIndent("", "  ")
		encoder.Encode(task.GetSnapshot())
	}
}
