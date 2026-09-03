package common

import (
	"bigdevops/src/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

type ArtifactoryItem struct {
	Uri          string            `json:"uri"`
	Path         string            `json:"path"`
	Folder       bool              `json:"folder"`
	Size         int64             `json:"size"`
	Created      string            `json:"created"`
	LastModified string            `json:"lastModified"`
	Children     []ArtifactoryItem `json:"children,omitempty"`
}

type ArtifactoryStorageResponse struct {
	Uri          string            `json:"uri"`
	Repo         string            `json:"repo"`
	Path         string            `json:"path"`
	Created      string            `json:"created"`
	LastModified string            `json:"lastModified"`
	Folder       bool              `json:"folder"`
	Children     []ArtifactoryItem `json:"children"`
	Size         string            `json:"size"`
}

type ArtifactoryClient struct {
	BaseURL  string
	Username string
	Password string
	Client   *http.Client
	Logger   *zap.Logger
}

func NewArtifactoryClient(sc *config.ServerConfig) (*ArtifactoryClient, error) {
	if sc.ArtifactoryC == nil || !sc.ArtifactoryC.Enable {
		return nil, fmt.Errorf("Artifactory 服务未启用，请在配置文件 server.yml 中配置 artifactory 部分")
	}

	baseURL := strings.TrimRight(sc.ArtifactoryC.BaseURL, "/")
	if !strings.HasSuffix(baseURL, "/artifactory") && !strings.Contains(baseURL, "/artifactory/") {
		baseURL = baseURL + "/artifactory"
	}
	return &ArtifactoryClient{
		BaseURL:  baseURL,
		Username: sc.ArtifactoryC.Username,
		Password: sc.ArtifactoryC.Password,
		Client:   &http.Client{Timeout: 30 * time.Second},
		Logger:   sc.Logger,
	}, nil
}

// SetAuth 为 HTTP 请求添加认证 (支持 BasicAuth、API Key 及 Bearer Token)
func (c *ArtifactoryClient) SetAuth(req *http.Request) {
	if c.Username != "" && c.Password != "" {
		req.SetBasicAuth(c.Username, c.Password)
		req.Header.Set("X-JFrog-Art-Api-Key", c.Password)
	} else if c.Password != "" {
		req.Header.Set("Authorization", "Bearer "+c.Password)
		req.Header.Set("X-JFrog-Art-Api-Key", c.Password)
	}
}

type ArtifactoryRepoInfo struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Description string `json:"description"`
	PackageType string `json:"packageType"`
	URL         string `json:"url"`
}

// GetRepositories 获取 Artifactory 全量仓库列表
func (c *ArtifactoryClient) GetRepositories() ([]ArtifactoryRepoInfo, error) {
	url := fmt.Sprintf("%s/api/repositories", c.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建获取仓库列表请求失败: %w", err)
	}
	c.SetAuth(req)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接到 Artifactory 获取仓库列表: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取仓库列表失败 [HTTP %d]: %s", resp.StatusCode, string(body))
	}

	var repos []ArtifactoryRepoInfo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("解析仓库列表 JSON 失败: %w", err)
	}

	return repos, nil
}

type ArtifactoryChecksums struct {
	Sha1   string `json:"sha1"`
	Sha256 string `json:"sha256"`
	Md5    string `json:"md5"`
}

type ArtifactoryFileDetail struct {
	Repo             string               `json:"repo"`
	Path             string               `json:"path"`
	RepoPath         string               `json:"repoPath"`
	DownloadURI      string               `json:"downloadUri"`
	Created          string               `json:"created"`
	CreatedBy        string               `json:"createdBy"`
	LastModified     string               `json:"lastModified"`
	ModifiedBy       string               `json:"modifiedBy"`
	Size             int64                `json:"size"`
	SizeFormatted    string               `json:"sizeFormatted"`
	MimeType         string               `json:"mimeType"`
	Checksums        ArtifactoryChecksums `json:"checksums"`
	Downloads        int64                `json:"downloads"`
	LastDownloaded   string               `json:"lastDownloaded"`
	LastDownloadedBy string               `json:"lastDownloadedBy"`
}

// GetFileInfo 获取指定文件/构件的详细元数据与统计数据
func (c *ArtifactoryClient) GetFileInfo(repo string, filePath string) (*ArtifactoryFileDetail, error) {
	filePath = strings.TrimLeft(filePath, "/")
	url := fmt.Sprintf("%s/api/storage/%s/%s", c.BaseURL, repo, filePath)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.SetAuth(req)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求文件元数据失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取文件元数据失败 [HTTP %d]: %s", resp.StatusCode, string(body))
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("解析文件元数据失败: %w", err)
	}

	detail := &ArtifactoryFileDetail{
		Repo:         repo,
		Path:         filePath,
		RepoPath:     fmt.Sprintf("%s/%s", repo, filePath),
		DownloadURI:  fmt.Sprintf("%s/%s/%s", c.BaseURL, repo, filePath),
		Created:      fmt.Sprintf("%v", raw["created"]),
		CreatedBy:    fmt.Sprintf("%v", raw["createdBy"]),
		LastModified: fmt.Sprintf("%v", raw["lastModified"]),
		ModifiedBy:   fmt.Sprintf("%v", raw["modifiedBy"]),
		MimeType:     fmt.Sprintf("%v", raw["mimeType"]),
	}

	if sz, ok := raw["size"]; ok {
		switch v := sz.(type) {
		case string:
			var sizeInt int64
			fmt.Sscanf(v, "%d", &sizeInt)
			detail.Size = sizeInt
		case float64:
			detail.Size = int64(v)
		}
	}
	detail.SizeFormatted = formatBytes(detail.Size)

	if cs, ok := raw["checksums"].(map[string]interface{}); ok {
		detail.Checksums = ArtifactoryChecksums{
			Sha1:   fmt.Sprintf("%v", cs["sha1"]),
			Sha256: fmt.Sprintf("%v", cs["sha256"]),
			Md5:    fmt.Sprintf("%v", cs["md5"]),
		}
	}

	// 额外获取下载统计 (stats)
	statsURL := fmt.Sprintf("%s/api/storage/%s/%s?stats", c.BaseURL, repo, filePath)
	if sReq, err := http.NewRequest("GET", statsURL, nil); err == nil {
		c.SetAuth(sReq)
		if sResp, err := c.Client.Do(sReq); err == nil && sResp.StatusCode == http.StatusOK {
			defer sResp.Body.Close()
			var statsMap map[string]interface{}
			if err := json.NewDecoder(sResp.Body).Decode(&statsMap); err == nil {
				if dl, ok := statsMap["downloadCount"].(float64); ok {
					detail.Downloads = int64(dl)
				}
				if dlBy, ok := statsMap["lastDownloadedBy"].(string); ok {
					detail.LastDownloadedBy = dlBy
				}
				if lastDl, ok := statsMap["lastDownloaded"].(float64); ok {
					detail.LastDownloaded = formatTimestamp(int64(lastDl))
				}
			}
		}
	}

	return detail, nil
}

func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const k = 1024
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	val := float64(bytes)
	for val >= k && i < len(sizes)-1 {
		val /= k
		i++
	}
	return fmt.Sprintf("%.2f %s", val, sizes[i])
}

func formatTimestamp(ms int64) string {
	if ms <= 0 {
		return "无记录"
	}
	t := time.Unix(ms/1000, 0)
	return t.Format("2006-01-02 15:04:05")
}

// GetFileTree 获取存储库指定路径下的元数据及子文件列表
func (c *ArtifactoryClient) GetFileTree(repo string, folderPath string) (*ArtifactoryStorageResponse, error) {
	folderPath = strings.Trim(folderPath, "/")
	var url string
	if folderPath == "" {
		url = fmt.Sprintf("%s/api/storage/%s", c.BaseURL, repo)
	} else {
		url = fmt.Sprintf("%s/api/storage/%s/%s", c.BaseURL, repo, folderPath)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	c.SetAuth(req)

	resp, err := c.Client.Do(req)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Error("[Artifactory] 请求失败", zap.String("url", url), zap.Error(err))
		}
		return nil, fmt.Errorf("无法连接到 Artifactory 服务地址 [%s]: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if c.Logger != nil {
			c.Logger.Error("[Artifactory] 接口返回非200状态码", zap.String("url", url), zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
		}
		return nil, fmt.Errorf("请求 [%s] 失败 [HTTP %d]: %s", url, resp.StatusCode, string(body))
	}

	var storageResp ArtifactoryStorageResponse
	if err := json.Unmarshal(body, &storageResp); err != nil {
		return nil, fmt.Errorf("解析 Artifactory 响应 JSON 失败: %w", err)
	}

	return &storageResp, nil
}

// GetFileContent 读取文本文件内容
func (c *ArtifactoryClient) GetFileContent(repo string, filePath string) ([]byte, error) {
	filePath = strings.TrimLeft(filePath, "/")
	url := fmt.Sprintf("%s/%s/%s", c.BaseURL, repo, filePath)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.SetAuth(req)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求文件内容失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("读取文件失败 [HTTP %d]: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// SaveFileContent 保存或更新文本文件内容 (PUT)
func (c *ArtifactoryClient) SaveFileContent(repo string, filePath string, content []byte) error {
	filePath = strings.TrimLeft(filePath, "/")
	url := fmt.Sprintf("%s/%s/%s", c.BaseURL, repo, filePath)

	req, err := http.NewRequest("PUT", url, bytes.NewReader(content))
	if err != nil {
		return err
	}
	c.SetAuth(req)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("保存文件失败 [HTTP %d]: %s", resp.StatusCode, string(body))
	}

	return nil
}

// UploadFile 上传文件 (流式上传)
func (c *ArtifactoryClient) UploadFile(repo string, filePath string, reader io.Reader, size int64) error {
	filePath = strings.TrimLeft(filePath, "/")
	url := fmt.Sprintf("%s/%s/%s", c.BaseURL, repo, filePath)

	req, err := http.NewRequest("PUT", url, reader)
	if err != nil {
		return err
	}
	c.SetAuth(req)
	if size > 0 {
		req.ContentLength = size
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("上传文件失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("上传文件失败 [HTTP %d]: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteFile 删除文件或目录
func (c *ArtifactoryClient) DeleteFile(repo string, filePath string) error {
	filePath = strings.TrimLeft(filePath, "/")
	url := fmt.Sprintf("%s/%s/%s", c.BaseURL, repo, filePath)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	c.SetAuth(req)

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("删除请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("删除失败 [HTTP %d]: %s", resp.StatusCode, string(body))
	}

	return nil
}
