package common

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bigdevops/src/config"

	"go.uber.org/zap"
)

// HarborProject Harbor 项目结构
type HarborProject struct {
	ProjectID    int                    `json:"project_id"`
	Name         string                 `json:"name"`
	RepoCount    int                    `json:"repo_count"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreationTime string                 `json:"creation_time"`
	UpdateTime   string                 `json:"update_time"`
}

// HarborRepo Harbor 镜像仓库结构
type HarborRepo struct {
	ID            int    `json:"id"`
	Name          string `json:"name"` // 格式如: bigdevops/server
	ProjectID     int    `json:"project_id"`
	ArtifactCount int    `json:"artifact_count"`
	PullCount     int    `json:"pull_count"`
	CreationTime  string `json:"creation_time"`
	UpdateTime    string `json:"update_time"`
}

// HarborTag 镜像 Tag 标签
type HarborTag struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	ArtifactID int    `json:"artifact_id"`
	PushTime   string `json:"push_time"`
	PullTime   string `json:"pull_time"`
	Signed     bool   `json:"signed"`
}

// HarborArtifact 镜像制品 Artifact
type HarborArtifact struct {
	ID         int                    `json:"id"`
	Type       string                 `json:"type"`
	Digest     string                 `json:"digest"`
	Size       int64                  `json:"size"`
	Tags       []*HarborTag           `json:"tags"`
	PushTime   string                 `json:"push_time"`
	PullTime   string                 `json:"pull_time"`
	ExtraAttrs map[string]interface{} `json:"extra_attrs"`
}

// HarborClient Harbor API 客户端
type HarborClient struct {
	BaseURL  string
	Username string
	Password string
	Client   *http.Client
	Logger   *zap.Logger
}

// NewHarborClient 创建 Harbor 客户端
func NewHarborClient(sc *config.ServerConfig) (*HarborClient, error) {
	if sc.ArtifactC == nil || sc.ArtifactC.Harbor == nil || !sc.ArtifactC.Harbor.Enable {
		return nil, fmt.Errorf("Harbor 镜像仓库未启用，请在配置文件中配置 artifact.harbor 部分")
	}

	baseURL := strings.TrimRight(sc.ArtifactC.Harbor.BaseURL, "/")
	if baseURL == "" {
		return nil, fmt.Errorf("Harbor base_url 不能为空")
	}

	// 支持跳过自签名证书校验，防止内网 https 报证书不受信
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &HarborClient{
		BaseURL:  baseURL,
		Username: sc.ArtifactC.Harbor.Username,
		Password: sc.ArtifactC.Harbor.Password,
		Client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: tr,
		},
		Logger: sc.Logger,
	}, nil
}

// setAuth 设置认证
func (c *HarborClient) setAuth(req *http.Request) {
	if c.Username != "" && c.Password != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
	req.Header.Set("Accept", "application/json")
}

// GetProjects 获取项目列表
func (c *HarborClient) GetProjects(page, pageSize int, name string) ([]HarborProject, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 100
	}

	apiURL := fmt.Sprintf("%s/api/v2.0/projects?page=%d&page_size=%d", c.BaseURL, page, pageSize)
	if name != "" {
		apiURL += fmt.Sprintf("&name=%s", url.QueryEscape(name))
	}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Harbor 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Harbor 返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var projects []HarborProject
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, fmt.Errorf("解析项目列表失败: %w", err)
	}
	return projects, nil
}

// GetRepositories 获取指定项目下的镜像仓库列表
func (c *HarborClient) GetRepositories(projectName string, page, pageSize int, query string) ([]HarborRepo, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 100
	}

	apiURL := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories?page=%d&page_size=%d",
		c.BaseURL, url.PathEscape(projectName), page, pageSize)
	if query != "" {
		apiURL += fmt.Sprintf("&q=%s", url.QueryEscape("name=~"+query))
	}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Harbor 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Harbor 返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var repos []HarborRepo
	if err := json.Unmarshal(body, &repos); err != nil {
		return nil, fmt.Errorf("解析镜像仓库列表失败: %w", err)
	}
	return repos, nil
}

// GetArtifacts 获取指定镜像仓库下的 Artifact / Tags 列表
func (c *HarborClient) GetArtifacts(projectName, repoName string, page, pageSize int) ([]HarborArtifact, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 100
	}

	// 处理 repoName：Harbor 仓库名称若包含项目前缀 (如 bigdevops/server)，需要截取纯 repo 名称
	shortRepoName := repoName
	if strings.HasPrefix(shortRepoName, projectName+"/") {
		shortRepoName = strings.TrimPrefix(shortRepoName, projectName+"/")
	}

	// Harbor v2.0 API: /projects/{project_name}/repositories/{repository_name}/artifacts
	// 针对短 repo 名称做 URL PathEscape
	apiURL := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s/artifacts?with_tag=true&page=%d&page_size=%d",
		c.BaseURL, url.PathEscape(projectName), url.PathEscape(shortRepoName), page, pageSize)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Harbor 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Harbor 返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var artifacts []HarborArtifact
	if err := json.Unmarshal(body, &artifacts); err != nil {
		return nil, fmt.Errorf("解析镜像制品失败: %w", err)
	}
	return artifacts, nil
}

// DeleteArtifact 删除指定镜像版本
func (c *HarborClient) DeleteArtifact(projectName, repoName, reference string) error {
	shortRepoName := repoName
	if strings.HasPrefix(shortRepoName, projectName+"/") {
		shortRepoName = strings.TrimPrefix(shortRepoName, projectName+"/")
	}

	apiURL := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s/artifacts/%s",
		c.BaseURL, url.PathEscape(projectName), url.PathEscape(shortRepoName), url.PathEscape(reference))

	req, err := http.NewRequest(http.MethodDelete, apiURL, nil)
	if err != nil {
		return err
	}
	c.setAuth(req)

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("请求 Harbor 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Harbor 删除失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// CreateProject 在 Harbor 中创建项目
func (c *HarborClient) CreateProject(name string, isPublic bool, storageLimit int64) error {
	apiURL := fmt.Sprintf("%s/api/v2.0/projects", c.BaseURL)

	publicStr := "false"
	if isPublic {
		publicStr = "true"
	}

	payload := map[string]interface{}{
		"project_name": name,
		"public":       isPublic,
		"metadata": map[string]interface{}{
			"public": publicStr,
		},
	}
	if storageLimit > 0 {
		payload["storage_limit"] = storageLimit
	} else {
		payload["storage_limit"] = -1 // 不限制
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(string(jsonBytes)))
	if err != nil {
		return err
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("请求 Harbor 创建项目失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("Harbor 项目 [%s] 已存在，请勿重复创建", name)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Harbor 创建项目失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
