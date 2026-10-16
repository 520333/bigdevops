package common

import (
	"bigdevops/src/config"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type ArcheryClient struct {
	BaseURL  string
	Username string
	Password string
	Client   *http.Client
	Logger   *zap.Logger

	tokenMutex sync.RWMutex
	token      string
	tokenExp   time.Time
}

func NewArcheryClient(sc *config.ServerConfig) (*ArcheryClient, error) {
	if sc.ArcheryC == nil || !sc.ArcheryC.Enable {
		return nil, fmt.Errorf("Archery 服务未启用，请在配置文件 server.yml 中配置 archery 部分")
	}

	baseURL := strings.TrimRight(sc.ArcheryC.BaseURL, "/")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &ArcheryClient{
		BaseURL:  baseURL,
		Username: sc.ArcheryC.Username,
		Password: sc.ArcheryC.Password,
		Client:   &http.Client{Timeout: 30 * time.Second, Transport: tr},
		Logger:   sc.Logger,
	}, nil
}

// GetToken 获取或自动刷新 Archery JWT 访问令牌
func (c *ArcheryClient) GetToken() (string, error) {
	c.tokenMutex.RLock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		token := c.token
		c.tokenMutex.RUnlock()
		return token, nil
	}
	c.tokenMutex.RUnlock()

	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()

	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}

	authURL := fmt.Sprintf("%s/api/auth/token/", c.BaseURL)
	payload := map[string]string{
		"username": c.Username,
		"password": c.Password,
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", authURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建登录认证请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接 Archery 认证接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Archery 认证失败 [HTTP %d]: %s", resp.StatusCode, string(respBody))
	}

	var authResp struct {
		Access  string `json:"access"`
		Refresh string `json:"refresh"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", fmt.Errorf("解析认证响应失败: %w", err)
	}

	if authResp.Access == "" {
		return "", fmt.Errorf("Archery 登录未返回有效的 access token")
	}

	c.token = authResp.Access
	c.tokenExp = time.Now().Add(4 * time.Hour) // JWT 默认有效期较长，缓存 4 小时
	return c.token, nil
}

// SetAuth 为 HTTP 请求附加 Authorization Bearer Token
func (c *ArcheryClient) SetAuth(req *http.Request) error {
	token, err := c.GetToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	return nil
}

// ArcheryInstance 数据库实例模型
type ArcheryInstance struct {
	ID            int    `json:"id"`
	InstanceName  string `json:"instance_name"`
	Type          string `json:"type"`
	DbType        string `json:"db_type"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	User          string `json:"user"`
	Charset       string `json:"charset"`
	ResourceGroup []int  `json:"resource_group"`
}

// ArcheryInstanceListResp 实例分页响应
type ArcheryInstanceListResp struct {
	Count   int               `json:"count"`
	Next    string            `json:"next"`
	Results []ArcheryInstance `json:"results"`
}

// GetInstances 获取数据库实例列表 (自动分页拉取全部实例)
func (c *ArcheryClient) GetInstances() ([]ArcheryInstance, error) {
	nextURL := fmt.Sprintf("%s/api/v1/instance/?size=1000", c.BaseURL)
	var allInstances []ArcheryInstance

	for nextURL != "" {
		req, err := http.NewRequest("GET", nextURL, nil)
		if err != nil {
			return nil, err
		}
		if err := c.SetAuth(req); err != nil {
			return nil, err
		}

		resp, err := c.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("获取数据库实例列表失败: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("获取数据库实例失败 [HTTP %d]: %s", resp.StatusCode, string(respBody))
		}

		var listResp ArcheryInstanceListResp
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("解析实例列表失败: %w", err)
		}
		resp.Body.Close()

		allInstances = append(allInstances, listResp.Results...)
		nextURL = listResp.Next
	}

	return allInstances, nil
}

// GetDatabases 获取指定实例下的数据库列表
func (c *ArcheryClient) GetDatabases(instanceID int) ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/instance/resource/", c.BaseURL)
	payload := map[string]interface{}{
		"instance_id":   instanceID,
		"resource_type": "database",
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.SetAuth(req); err != nil {
		return nil, err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取数据库列表失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取数据库失败 [HTTP %d]: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Count  int         `json:"count"`
		Result []string    `json:"result"`
		Data   interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("解析数据库列表响应失败: %w", err)
	}

	if len(res.Result) > 0 {
		return res.Result, nil
	}

	// 兼容 data 字段返回数组的情况
	if res.Data != nil {
		if dList, ok := res.Data.([]interface{}); ok {
			var strList []string
			for _, item := range dList {
				strList = append(strList, fmt.Sprintf("%v", item))
			}
			return strList, nil
		}
	}

	return []string{}, nil
}

// ArcherySqlCheckRow 单条 SQL 审核诊断详情
type ArcherySqlCheckRow struct {
	ID           int    `json:"id"`
	Stage        string `json:"stage"`
	ErrLevel     int    `json:"errlevel"` // 0: 正常通过, 1: 警告, 2: 错误
	StageStatus  string `json:"stagestatus"`
	ErrorMessage string `json:"errormessage"`
	Sql          string `json:"sql"`
	AffectedRows int64  `json:"affected_rows"`
}

// ArcherySqlCheckResult SQL 预审核诊断结果
type ArcherySqlCheckResult struct {
	IsExecute    bool                 `json:"is_execute"`
	Checked      string               `json:"checked"`
	WarningCount int                  `json:"warning_count"`
	ErrorCount   int                  `json:"error_count"`
	IsCritical   bool                 `json:"is_critical"`
	Rows         []ArcherySqlCheckRow `json:"rows"`
}

// SqlCheck 基于 Inception 对待上线的 SQL 执行语法检测与合规预审核
func (c *ArcheryClient) SqlCheck(instanceID int, dbName, fullSql string) (*ArcherySqlCheckResult, error) {
	url := fmt.Sprintf("%s/api/v1/workflow/sqlcheck/", c.BaseURL)
	payload := map[string]interface{}{
		"instance_id": instanceID,
		"db_name":     dbName,
		"full_sql":    fullSql,
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.SetAuth(req); err != nil {
		return nil, err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用 SQL 检测接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SQL 检测失败 [HTTP %d]: %s", resp.StatusCode, string(respBody))
	}

	var checkResult ArcherySqlCheckResult
	if err := json.NewDecoder(resp.Body).Decode(&checkResult); err != nil {
		return nil, fmt.Errorf("解析 SQL 检测结果失败: %w", err)
	}

	return &checkResult, nil
}

// ArcheryWorkflowDetail 工单内部属性
type ArcheryWorkflowDetail struct {
	ID              int    `json:"id"`
	WorkflowName    string `json:"workflow_name"`
	DemandUrl       string `json:"demand_url"`
	GroupId         int    `json:"group_id"`
	GroupName       string `json:"group_name"`
	Instance        int    `json:"instance"`
	DbName          string `json:"db_name"`
	SyntaxType      int    `json:"syntax_type"` // 1: DDL, 2: DML
	IsBackup        bool   `json:"is_backup"`
	Engineer        string `json:"engineer"`
	EngineerDisplay string `json:"engineer_display"`
	Status          string `json:"status"`
	AuditAuthGroups string `json:"audit_auth_groups"`
	CreateTime      string `json:"create_time"`
	FinishTime      string `json:"finish_time"`
}

// ArcheryWorkflowItem 工单条目
type ArcheryWorkflowItem struct {
	ID            int                   `json:"id"`
	WorkflowId    int                   `json:"workflow_id"`
	Workflow      ArcheryWorkflowDetail `json:"workflow"`
	SqlContent    string                `json:"sql_content"`
	ReviewContent string                `json:"review_content"`
	ExecuteResult string                `json:"execute_result"`
}

// ArcheryWorkflowListResp 工单列表响应
type ArcheryWorkflowListResp struct {
	Count   int                   `json:"count"`
	Next    string                `json:"next"`
	Results []ArcheryWorkflowItem `json:"results"`
}

// ArcheryGroupItem Archery 组信息
type ArcheryGroupItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// fetchResourceGroupsFromSubmitsql 从 Archery 原生 /submitsql/ 动态提取完整的资源组列表（完全对齐原生「搜索您所在的组」）
func (c *ArcheryClient) fetchResourceGroupsFromSubmitsql() ([]ArcheryGroupItem, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	webClient := &http.Client{
		Jar:       jar,
		Timeout:   10 * time.Second,
		Transport: c.Client.Transport,
	}

	// 1. 获取登录页 CSRF Token
	loginURL := fmt.Sprintf("%s/login/", c.BaseURL)
	loginResp, err := webClient.Get(loginURL)
	if err != nil {
		return nil, err
	}
	defer loginResp.Body.Close()

	var csrfToken string
	for _, cookie := range loginResp.Cookies() {
		if cookie.Name == "csrftoken" {
			csrfToken = cookie.Value
			break
		}
	}

	// 2. 模拟登录获取会话
	authURL := fmt.Sprintf("%s/authenticate/", c.BaseURL)
	loginData := url.Values{
		"username":            {c.Username},
		"password":            {c.Password},
		"csrfmiddlewaretoken": {csrfToken},
	}
	authReq, err := http.NewRequest("POST", authURL, strings.NewReader(loginData.Encode()))
	if err != nil {
		return nil, err
	}
	authReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	authReq.Header.Set("Referer", loginURL)
	authReq.Header.Set("X-CSRFToken", csrfToken)

	authResp, err := webClient.Do(authReq)
	if err != nil {
		return nil, err
	}
	authResp.Body.Close()

	// 3. 请求 /submitsql/ 页面解析所属组下拉框
	submitURL := fmt.Sprintf("%s/submitsql/", c.BaseURL)
	submitReq, err := http.NewRequest("GET", submitURL, nil)
	if err != nil {
		return nil, err
	}
	submitResp, err := webClient.Do(submitReq)
	if err != nil {
		return nil, err
	}
	defer submitResp.Body.Close()

	bodyBytes, err := io.ReadAll(submitResp.Body)
	if err != nil {
		return nil, err
	}

	reOption := regexp.MustCompile(`<option[^>]*group-id=["'](\d+)["'][^>]*>([^<]+)</option>`)
	matches := reOption.FindAllStringSubmatch(string(bodyBytes), -1)

	var list []ArcheryGroupItem
	seen := make(map[int]bool)
	for _, m := range matches {
		id, _ := strconv.Atoi(m[1])
		name := strings.TrimSpace(m[2])
		if id > 0 && name != "" && !seen[id] {
			seen[id] = true
			list = append(list, ArcheryGroupItem{
				ID:   id,
				Name: name,
			})
		}
	}

	return list, nil
}

// GetGroups 获取 Archery 组列表 (优先获取 /submitsql/ 原生全部资源组，并聚合补充)
func (c *ArcheryClient) GetGroups() ([]ArcheryGroupItem, error) {
	// 1. 优先获取原生资源组列表（完全对齐 Archery 原生下拉选项与顺序）
	nativeGroups, err := c.fetchResourceGroupsFromSubmitsql()
	if err == nil && len(nativeGroups) > 0 {
		return nativeGroups, nil
	}

	// 2. 降级备用方案：聚合 /api/v1/user/group/ 及工单历史字典
	groupMap := make(map[int]string)

	reqUrl := fmt.Sprintf("%s/api/v1/user/group/?size=100", c.BaseURL)
	req, errReq := http.NewRequest("GET", reqUrl, nil)
	if errReq == nil && c.SetAuth(req) == nil {
		if resp, errDo := c.Client.Do(req); errDo == nil {
			var res struct {
				Results []ArcheryGroupItem `json:"results"`
			}
			if json.NewDecoder(resp.Body).Decode(&res) == nil {
				for _, g := range res.Results {
					if g.ID > 0 && g.Name != "" {
						groupMap[g.ID] = g.Name
					}
				}
			}
			resp.Body.Close()
		}
	}

	reqWfUrl := fmt.Sprintf("%s/api/v1/workflow/?size=100", c.BaseURL)
	reqWf, errWf := http.NewRequest("GET", reqWfUrl, nil)
	if errWf == nil && c.SetAuth(reqWf) == nil {
		if respWf, errWfDo := c.Client.Do(reqWf); errWfDo == nil {
			var resWf struct {
				Results []struct {
					Workflow struct {
						GroupId   int    `json:"group_id"`
						GroupName string `json:"group_name"`
					} `json:"workflow"`
				} `json:"results"`
			}
			if json.NewDecoder(respWf.Body).Decode(&resWf) == nil {
				for _, it := range resWf.Results {
					if it.Workflow.GroupId > 0 && it.Workflow.GroupName != "" {
						groupMap[it.Workflow.GroupId] = it.Workflow.GroupName
					}
				}
			}
			respWf.Body.Close()
		}
	}

	var list []ArcheryGroupItem
	for id, name := range groupMap {
		list = append(list, ArcheryGroupItem{ID: id, Name: name})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return list, nil
}

// ArcheryWorkflowFilter 工单查询多维过滤参数
type ArcheryWorkflowFilter struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
	Search     string `json:"search"`
	Status     string `json:"status"`
	InstanceId int    `json:"instanceId"`
	GroupId    int    `json:"groupId"`
	StartTime  string `json:"startTime"`
	EndTime    string `json:"endTime"`
}

// buildFilterQuery 构造 Archery DRF 兼容的查询过滤参数
func (c *ArcheryClient) buildFilterQuery(f ArcheryWorkflowFilter) string {
	var q []string
	if f.Search != "" {
		q = append(q, fmt.Sprintf("workflow__workflow_name__icontains=%s", url.QueryEscape(f.Search)))
	}
	if f.Status != "" {
		q = append(q, fmt.Sprintf("workflow__status=%s", url.QueryEscape(f.Status)))
	}
	if f.InstanceId > 0 {
		q = append(q, fmt.Sprintf("workflow__instance_id=%d", f.InstanceId))
	}
	if f.StartTime != "" {
		st := strings.Replace(f.StartTime, " ", "T", 1)
		q = append(q, fmt.Sprintf("workflow__create_time__gte=%s", url.QueryEscape(st)))
	}
	if f.EndTime != "" {
		et := strings.Replace(f.EndTime, " ", "T", 1)
		if len(et) == 10 {
			if t, err := time.Parse("2006-01-02", et); err == nil {
				et = t.AddDate(0, 0, 1).Format("2006-01-02")
			}
		}
		q = append(q, fmt.Sprintf("workflow__create_time__lt=%s", url.QueryEscape(et)))
	}
	if len(q) > 0 {
		return "&" + strings.Join(q, "&")
	}
	return ""
}

// fetchWorkflowPage 内部拉取 Archery 指定页
func (c *ArcheryClient) fetchWorkflowPage(page, size int, extraQuery string) (*ArcheryWorkflowListResp, error) {
	reqUrl := fmt.Sprintf("%s/api/v1/workflow/?page=%d&size=%d%s", c.BaseURL, page, size, extraQuery)

	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		return nil, err
	}
	if err := c.SetAuth(req); err != nil {
		return nil, err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取工单列表失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取工单列表失败 [HTTP %d]: %s", resp.StatusCode, string(respBody))
	}

	var listResp ArcheryWorkflowListResp
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("解析工单列表失败: %w", err)
	}

	return &listResp, nil
}

// GetWorkflows 获取工单列表（支持多维度过滤、精准倒序映射）
// 筛选维度支持：状态(status)、实例(instanceId)、组(groupId)、时间跨度(startTime/endTime)、搜索关键词(search)
func (c *ArcheryClient) GetWorkflows(filter ArcheryWorkflowFilter) (*ArcheryWorkflowListResp, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	extraQuery := c.buildFilterQuery(filter)

	// 1. 先探查总数量 (使用 size=1 最小化网络传输)
	probe, err := c.fetchWorkflowPage(1, 1, extraQuery)
	if err != nil {
		return nil, err
	}
	total := probe.Count
	if total == 0 {
		return &ArcheryWorkflowListResp{Count: 0, Results: []ArcheryWorkflowItem{}}, nil
	}

	// 2. 计算按时间倒序（最新工单在第 1 页）在 Archery 正序数据源中的切片范围
	startDesc := (page - 1) * pageSize
	if startDesc >= total {
		return &ArcheryWorkflowListResp{Count: total, Results: []ArcheryWorkflowItem{}}, nil
	}
	endDesc := startDesc + pageSize
	if endDesc > total {
		endDesc = total
	}

	// 映射为 Archery 正序索引 (0-indexed: [startAsc, endAsc))
	startAsc := total - endDesc
	endAsc := total - startDesc

	// 计算覆盖该区间所需的 Archery 页号 (1-indexed)
	p1 := (startAsc / pageSize) + 1
	p2 := ((endAsc - 1) / pageSize) + 1

	var combined []ArcheryWorkflowItem
	offsetP1 := (p1 - 1) * pageSize

	resp1, err := c.fetchWorkflowPage(p1, pageSize, extraQuery)
	if err != nil {
		return nil, err
	}
	combined = append(combined, resp1.Results...)

	if p2 > p1 {
		resp2, err := c.fetchWorkflowPage(p2, pageSize, extraQuery)
		if err == nil {
			combined = append(combined, resp2.Results...)
		}
	}

	// 在 combined 中提取对应切片
	sliceStart := startAsc - offsetP1
	sliceEnd := endAsc - offsetP1

	if sliceStart < 0 {
		sliceStart = 0
	}
	if sliceEnd > len(combined) {
		sliceEnd = len(combined)
	}
	if sliceStart > sliceEnd {
		sliceStart = sliceEnd
	}

	selected := combined[sliceStart:sliceEnd]

	// 翻转为降序，使最新工单排在最前面
	reversed := make([]ArcheryWorkflowItem, len(selected))
	for i, item := range selected {
		reversed[len(selected)-1-i] = item
	}

	// 若传入了 groupId，做内存精准比对过滤
	if filter.GroupId > 0 {
		var filtered []ArcheryWorkflowItem
		for _, it := range reversed {
			if it.Workflow.GroupId == filter.GroupId {
				filtered = append(filtered, it)
			}
		}
		reversed = filtered
	}

	return &ArcheryWorkflowListResp{
		Count:   total,
		Results: reversed,
	}, nil
}

// ArcheryCreateWorkflowReq 提交工单请求参数
type ArcheryCreateWorkflowReq struct {
	WorkflowName string `json:"workflowName" binding:"required"`
	InstanceId   int    `json:"instanceId" binding:"required"`
	DbName       string `json:"dbName" binding:"required"`
	GroupId      int    `json:"groupId"`
	IsBackup     bool   `json:"isBackup"`
	DemandUrl    string `json:"demandUrl"`
	SqlContent   string `json:"sqlContent" binding:"required"`
	Engineer     string `json:"engineer"`
}

// CreateWorkflow 提交 SQL 上线工单
func (c *ArcheryClient) CreateWorkflow(req ArcheryCreateWorkflowReq) (int, error) {
	url := fmt.Sprintf("%s/api/v1/workflow/", c.BaseURL)

	groupId := req.GroupId
	if groupId <= 0 {
		// 若前端未传入资源组，尝试从该实例的 resource_group 列表获取默认组
		instances, err := c.GetInstances()
		if err == nil {
			for _, inst := range instances {
				if inst.ID == req.InstanceId && len(inst.ResourceGroup) > 0 {
					groupId = inst.ResourceGroup[0]
					break
				}
			}
		}
		if groupId <= 0 {
			groupId = 1 // 默认回退到组 1
		}
	}

	engineer := req.Engineer
	if engineer == "" {
		engineer = c.Username
	}

	payload := map[string]interface{}{
		"workflow": map[string]interface{}{
			"workflow_name":     req.WorkflowName,
			"group_id":          groupId,
			"instance":          req.InstanceId,
			"db_name":           req.DbName,
			"is_backup":         req.IsBackup,
			"is_offline_export": 0,
			"demand_url":        req.DemandUrl,
			"engineer":          engineer,
		},
		"sql_content": req.SqlContent,
	}

	bodyBytes, _ := json.Marshal(payload)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err := c.SetAuth(httpReq); err != nil {
		return 0, err
	}

	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("提交工单失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("提交工单失败 [HTTP %d]: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		ID         int `json:"id"`
		WorkflowId int `json:"workflow_id"`
	}
	_ = json.Unmarshal(respBody, &res)

	wfID := res.WorkflowId
	if wfID == 0 {
		wfID = res.ID
	}
	return wfID, nil
}

// AuditWorkflow 审核工单 (auditType: "pass" 通过, "cancel" 终止/驳回)
func (c *ArcheryClient) AuditWorkflow(workflowId int, auditType, auditRemark, engineer string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/workflow/audit/", c.BaseURL)
	if engineer == "" {
		engineer = c.Username
	}
	if auditRemark == "" {
		if auditType == "pass" {
			auditRemark = "审核通过"
		} else {
			auditRemark = "终止工单"
		}
	}

	payload := map[string]interface{}{
		"workflow_id":   workflowId,
		"workflow_type": 2, // 2: SQL上线工单
		"audit_type":    auditType,
		"audit_remark":  auditRemark,
		"engineer":      engineer,
	}

	bodyBytes, _ := json.Marshal(payload)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err := c.SetAuth(httpReq); err != nil {
		return "", err
	}

	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("审核工单失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("审核工单失败 [HTTP %d]: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Msg string `json:"msg"`
	}
	_ = json.Unmarshal(respBody, &res)
	msg := res.Msg
	if msg == "" {
		msg = string(respBody)
	}
	return msg, nil
}

// ExecuteWorkflow 执行上线工单 (mode: "auto" 立即执行, "manual" 手动执行)
func (c *ArcheryClient) ExecuteWorkflow(workflowId int, mode, engineer string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/workflow/execute/", c.BaseURL)
	if engineer == "" {
		engineer = c.Username
	}
	if mode == "" {
		mode = "auto"
	}

	payload := map[string]interface{}{
		"workflow_id":   workflowId,
		"workflow_type": 2,
		"mode":          mode,
		"engineer":      engineer,
	}

	bodyBytes, _ := json.Marshal(payload)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err := c.SetAuth(httpReq); err != nil {
		return "", err
	}

	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("执行工单失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("执行工单失败 [HTTP %d]: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Msg string `json:"msg"`
	}
	_ = json.Unmarshal(respBody, &res)
	msg := res.Msg
	if msg == "" {
		msg = string(respBody)
	}
	return msg, nil
}

type ArcheryWorkflowLogItem struct {
	OperationTypeDesc string `json:"operation_type_desc"`
	OperationInfo     string `json:"operation_info"`
	OperatorDisplay   string `json:"operator_display"`
	OperationTime     string `json:"operation_time"`
}

type ArcheryWorkflowLogResp struct {
	Count   int                      `json:"count"`
	Results []ArcheryWorkflowLogItem `json:"results"`
}

// GetWorkflowLogs 获取指定工单的流转操作日志 (Archery POST /api/v1/workflow/log/)
func (c *ArcheryClient) GetWorkflowLogs(workflowId int) ([]ArcheryWorkflowLogItem, error) {
	reqUrl := fmt.Sprintf("%s/api/v1/workflow/log/", c.BaseURL)
	payload := map[string]interface{}{
		"workflow_id":   workflowId,
		"workflow_type": 2,
	}

	bodyBytes, _ := json.Marshal(payload)
	httpReq, err := http.NewRequest("POST", reqUrl, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err := c.SetAuth(httpReq); err != nil {
		return nil, err
	}

	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("获取工单日志失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取工单日志失败 [HTTP %d]: %s", resp.StatusCode, string(respBody))
	}

	var logResp ArcheryWorkflowLogResp
	if err := json.NewDecoder(resp.Body).Decode(&logResp); err != nil {
		return nil, fmt.Errorf("解析工单日志失败: %w", err)
	}

	return logResp.Results, nil
}

// GetWorkflowDetail 获取单个工单详情 (GET /api/v1/workflow/<id>/)
func (c *ArcheryClient) GetWorkflowDetail(workflowId int) (*ArcheryWorkflowItem, error) {
	reqUrl := fmt.Sprintf("%s/api/v1/workflow/%d/", c.BaseURL, workflowId)
	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		return nil, err
	}
	if err := c.SetAuth(req); err != nil {
		return nil, err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取工单详情失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取工单详情失败 [HTTP %d]: %s", resp.StatusCode, string(respBody))
	}

	var item ArcheryWorkflowItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("解析工单详情失败: %w", err)
	}

	return &item, nil
}

// IsProductionWorkflow 判断工单是否属于生产环境
func (c *ArcheryClient) IsProductionWorkflow(wf *ArcheryWorkflowDetail) bool {
	if wf == nil {
		return false
	}
	combined := strings.ToLower(fmt.Sprintf("%s %s %s %s", wf.GroupName, wf.WorkflowName, wf.DbName, wf.AuditAuthGroups))
	// 包含明确的生产标识关键字
	if strings.Contains(combined, "prod") || strings.Contains(combined, "prd") || strings.Contains(combined, "生产") || strings.Contains(combined, "release") || strings.Contains(combined, "online") {
		return true
	}
	// 明确包含测试标识关键字
	if strings.Contains(combined, "test") || strings.Contains(combined, "dev") || strings.Contains(combined, "uat") || strings.Contains(combined, "qa") || strings.Contains(combined, "测试") || strings.Contains(combined, "开发") {
		return false
	}
	return false
}

// ArcheryUserDetail Archery 用户信息与资源组关联
type ArcheryUserDetail struct {
	ID            int    `json:"id"`
	Username      string `json:"username"`
	IsSuperuser   bool   `json:"is_superuser"`
	ResourceGroup []int  `json:"resource_group"`
}

// GetUserDetail 获取 Archery 中指定用户的详情及关联的资源组列表
func (c *ArcheryClient) GetUserDetail(username string) (*ArcheryUserDetail, error) {
	if username == "" {
		return nil, fmt.Errorf("username 不能为空")
	}
	reqUrl := fmt.Sprintf("%s/api/v1/user/?username=%s", c.BaseURL, url.QueryEscape(username))
	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		return nil, err
	}
	if err := c.SetAuth(req); err != nil {
		return nil, err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取 Archery 用户详情失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取 Archery 用户详情失败 [HTTP %d]: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Count   int                 `json:"count"`
		Results []ArcheryUserDetail `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("解析 Archery 用户详情响应失败: %w", err)
	}

	if len(res.Results) == 0 {
		return nil, nil
	}

	return &res.Results[0], nil
}
