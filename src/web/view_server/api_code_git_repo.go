package view_server

import (
	"crypto/tls"
	"fmt"

	"net/http"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/gin-gonic/gin"
	"github.com/xanzy/go-gitlab"

	"bigdevops/src/common"
	"bigdevops/src/models"
)

func getGitLabClient(server *models.CodeGitServer) (*gitlab.Client, error) {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	var transport http.RoundTripper = http.DefaultTransport
	if server.SkipVerify {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	httpClient.Transport = transport

	return gitlab.NewClient(server.Token, gitlab.WithBaseURL(server.Endpoint), gitlab.WithHTTPClient(httpClient))
}

func getGiteaClient(server *models.CodeGitServer) (*gitea.Client, error) {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	var transport http.RoundTripper = http.DefaultTransport
	if server.SkipVerify {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	httpClient.Transport = transport

	return gitea.NewClient(server.Endpoint, gitea.SetToken(server.Token), gitea.SetHTTPClient(httpClient))
}

// @Summary      创建Git代码仓库
// @Description  创建Git代码仓库 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "创建Git代码仓库 响应结果"
// @Router       /code/createCodeGitRepo [post]
// @Security     Bearer
func createCodeGitRepo(c *gin.Context) {
	var reqObj models.CodeGitRepo
	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.ReqBadFailWithMessage("参数解析错误", c)
		return
	}

	if reqObj.Name == "" || reqObj.ServerID == 0 {
		common.ReqBadFailWithMessage("仓库名和所属Git实例不能为空", c)
		return
	}

	if reqObj.Visibility == "" {
		reqObj.Visibility = "private"
	}

	server, err := models.GetCodeGitServerById(int(reqObj.ServerID))
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	_, _, _, err = createRemoteRepo(server, &reqObj)
	if err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("创建成功", c)
}

// @Summary      更新Git代码仓库配置
// @Description  更新Git代码仓库配置 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "更新Git代码仓库配置 响应结果"
// @Router       /code/updateCodeGitRepo [post]
// @Security     Bearer
func updateCodeGitRepo(c *gin.Context) {
	var reqObj models.CodeGitRepo

	if err := c.ShouldBindJSON(&reqObj); err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}

	if reqObj.ID == 0 || reqObj.FullName == "" {
		common.ReqBadFailWithMessage("更新失败：缺少记录ID或FullName", c)
		return
	}

	server, err := models.GetCodeGitServerById(int(reqObj.ServerID))
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	// 构造一个 oldRepo 用于 updateRemoteRepo 取出 FullName 等信息
	oldRepo := &models.CodeGitRepo{
		FullName: reqObj.FullName,
	}

	err = updateRemoteRepo(server, oldRepo, &reqObj)
	if err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}

	common.OkWithMessage("更新成功", c)
}

// @Summary      获取Git代码仓库列表
// @Description  获取Git代码仓库列表 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取Git代码仓库列表 响应结果"
// @Router       /code/getCodeGitRepoList [get]
// @Security     Bearer
func getCodeGitRepoList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	repoName := c.DefaultQuery("name", "")
	serverId, _ := strconv.Atoi(c.DefaultQuery("serverId", ""))
	visibility := c.DefaultQuery("visibility", "")
	namespace := c.DefaultQuery("namespace", "")

	if serverId == 0 {
		common.OkWithDetailed(gin.H{"items": []models.CodeGitRepo{}, "total": 0}, "请先选择所属Git实例", c)
		return
	}

	server, err := models.GetCodeGitServerById(serverId)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	objs := make([]models.CodeGitRepo, 0)
	var total int64 = 0

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		var projects []*gitlab.Project
		if repoName != "" || namespace != "" {
			opt := &gitlab.ListProjectsOptions{
				ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1},
			}
			if visibility != "" {
				opt.Visibility = gitlab.Ptr(gitlab.VisibilityValue(visibility))
			}
			var allProjects []*gitlab.Project
			for {
				ps, resp, err := client.Projects.ListProjects(opt)
				if err != nil {
					break
				}
				for _, p := range ps {
					matchName := true
					if repoName != "" {
						if !strings.Contains(strings.ToLower(p.Name), strings.ToLower(repoName)) &&
							!strings.Contains(strings.ToLower(p.PathWithNamespace), strings.ToLower(repoName)) {
							matchName = false
						}
					}
					matchNs := true
					if namespace != "" {
						if p.Namespace == nil || !strings.Contains(strings.ToLower(p.Namespace.FullPath), strings.ToLower(namespace)) {
							matchNs = false
						}
					}
					if matchName && matchNs {
						allProjects = append(allProjects, p)
					}
				}
				if resp == nil || resp.NextPage == 0 {
					break
				}
				opt.Page = resp.NextPage
			}
			total = int64(len(allProjects))
			start := (page - 1) * pageSize
			end := start + pageSize
			if start > len(allProjects) {
				start = len(allProjects)
			}
			if end > len(allProjects) {
				end = len(allProjects)
			}
			projects = allProjects[start:end]
		} else {
			opt := &gitlab.ListProjectsOptions{
				ListOptions: gitlab.ListOptions{
					Page:    page,
					PerPage: pageSize,
				},
			}
			if visibility != "" {
				opt.Visibility = gitlab.Ptr(gitlab.VisibilityValue(visibility))
			}
			ps, resp, err := client.Projects.ListProjects(opt)
			if err != nil {
				common.FailWithMessage(fmt.Sprintf("GitLab API 获取仓库失败: %v", err), c)
				return
			}
			projects = ps
			if resp != nil {
				total = int64(resp.TotalItems)
			}
		}

		creatorCache := make(map[int]string)

		for _, p := range projects {
			nsPath := ""
			if p.Namespace != nil {
				nsPath = p.Namespace.FullPath
			}
			obj := models.CodeGitRepo{
				ServerID:      uint(serverId),
				ProjectID:     p.ID,
				Name:          p.Name,
				FullName:      p.PathWithNamespace,
				NamespacePath: nsPath,
				Description:   p.Description,
				Visibility:    string(p.Visibility),
				CloneUrlHttp:  p.HTTPURLToRepo,
				CloneUrlSsh:   p.SSHURLToRepo,
				WebUrl:        p.WebURL,
				DefaultBranch: p.DefaultBranch,
			}
			obj.ID = uint(p.ID)

			creatorName := ""
			if p.CreatorID > 0 {
				if name, ok := creatorCache[p.CreatorID]; ok {
					creatorName = name
				} else {
					u, _, err := client.Users.GetUser(p.CreatorID, gitlab.GetUsersOptions{})
					if err == nil && u != nil {
						creatorName = u.Name
						creatorCache[p.CreatorID] = u.Name
					}
				}
			} else if p.Owner != nil && p.Owner.Name != "" {
				creatorName = p.Owner.Name
			} else if p.Namespace != nil && p.Namespace.Kind == "user" {
				creatorName = p.Namespace.Name
			}
			obj.CreateUserName = creatorName

			ownerName := ""
			if p.Owner != nil && p.Owner.Name != "" {
				ownerName = p.Owner.Name
			} else if p.Namespace != nil {
				ownerName = p.Namespace.Name
			}
			obj.OwnerName = ownerName

			obj.FillFrontAllData()
			objs = append(objs, obj)
		}
	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		var repos []*gitea.Repository
		if namespace != "" {
			searchOpt := gitea.SearchRepoOptions{
				Keyword: repoName,
				ListOptions: gitea.ListOptions{
					Page:     1,
					PageSize: 100,
				},
			}
			if visibility == "private" {
				b := true
				searchOpt.IsPrivate = &b
			} else if visibility == "public" {
				b := false
				searchOpt.IsPrivate = &b
			}
			var allRepos []*gitea.Repository
			for {
				rs, _, err := client.SearchRepos(searchOpt)
				if err != nil {
					break
				}
				for _, r := range rs {
					matchNs := true
					if namespace != "" {
						if r.Owner == nil || !strings.Contains(strings.ToLower(r.Owner.UserName), strings.ToLower(namespace)) {
							matchNs = false
						}
					}
					if matchNs {
						allRepos = append(allRepos, r)
					}
				}
				if len(rs) < 100 {
					break
				}
				searchOpt.Page++
			}
			total = int64(len(allRepos))
			start := (page - 1) * pageSize
			end := start + pageSize
			if start > len(allRepos) {
				start = len(allRepos)
			}
			if end > len(allRepos) {
				end = len(allRepos)
			}
			repos = allRepos[start:end]
		} else {
			searchOpt := gitea.SearchRepoOptions{
				Keyword: repoName,
				ListOptions: gitea.ListOptions{
					Page:     page,
					PageSize: pageSize,
				},
			}
			if visibility == "private" {
				b := true
				searchOpt.IsPrivate = &b
			} else if visibility == "public" {
				b := false
				searchOpt.IsPrivate = &b
			}

			rs, resp, err := client.SearchRepos(searchOpt)
			if err != nil {
				common.FailWithMessage(fmt.Sprintf("Gitea API 获取仓库失败: %v", err), c)
				return
			}
			repos = rs

			if resp != nil && resp.Header.Get("X-Total-Count") != "" {
				t, _ := strconv.ParseInt(resp.Header.Get("X-Total-Count"), 10, 64)
				total = t
			} else {
				total = int64(len(repos))
			}
		}

		for _, r := range repos {

			vis := "public"
			if r.Private {
				vis = "private"
			}
			ns := ""
			if r.Owner != nil {
				ns = r.Owner.UserName
			}
			obj := models.CodeGitRepo{
				ServerID:      uint(serverId),
				ProjectID:     int(r.ID),
				Name:          r.Name,
				FullName:      r.FullName,
				NamespacePath: ns,
				Description:   r.Description,
				Visibility:    vis,
				CloneUrlHttp:  r.CloneURL,
				CloneUrlSsh:   r.SSHURL,
				WebUrl:        r.HTMLURL,
				DefaultBranch: r.DefaultBranch,
			}
			obj.ID = uint(r.ID)

			creatorName := ""
			if r.Owner != nil {
				creatorName = r.Owner.UserName
				if r.Owner.FullName != "" {
					creatorName = r.Owner.FullName
				}
			}
			obj.CreateUserName = creatorName

			ownerName := ""
			if r.Owner != nil {
				ownerName = r.Owner.FullName
				if ownerName == "" {
					ownerName = r.Owner.UserName
				}
			}
			obj.OwnerName = ownerName

			obj.FillFrontAllData()
			objs = append(objs, obj)
		}
	}

	common.OkWithDetailed(gin.H{
		"items": objs,
		"total": total,
	}, "获取成功", c)
}

// GiteaRepoResp Gitea 仓库响应结构
type GiteaRepoResp struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Internal      bool   `json:"internal"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
}

// GitLabProjectResp Gitea 项目响应结构
type GitLabProjectResp struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"`
	Description       string `json:"description"`
	Visibility        string `json:"visibility"` // "private", "internal", "public"
	HTTPURLToRepo     string `json:"http_url_to_repo"`
	SSHURLToRepo      string `json:"ssh_url_to_repo"`
	WebURL            string `json:"web_url"`
	DefaultBranch     string `json:"default_branch"`
}

// 统一返回的成员结构体
type UnifiedMember struct {
	Username    string `json:"username"`
	AccessLevel int    `json:"accessLevel"` // 1:读 (Reporter/Read), 2:写 (Developer/Write), 3:管理 (Maintainer/Admin)
	AvatarURL   string `json:"avatarUrl"`
}

// 前端传来的请求参数
type MemberReq struct {
	RepoID      int    `json:"repoId"`
	ServerID    int    `json:"serverId" binding:"required"`
	FullName    string `json:"fullName"` // Gitea 使用
	Username    string `json:"username" binding:"required"`
	AccessLevel int    `json:"accessLevel"` // 删除时不需要这个字段，添加时需要
}

// GitLabMemberResp GitLab 成员响应
type GitLabMemberResp struct {
	Username    string `json:"username"`
	AvatarURL   string `json:"avatar_url"`
	AccessLevel int    `json:"access_level"`
}

// GitLabUserResp GitLab 用户搜索响应
type GitLabUserResp struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// GiteaMemberResp Gitea 成员响应
type GiteaMemberResp struct {
	UserName    string `json:"login"`
	AvatarUrl   string `json:"avatar_url"`
	Permissions struct {
		Admin bool `json:"admin"`
		Push  bool `json:"push"`
		Pull  bool `json:"pull"`
	} `json:"permissions"`
}

// mapRoleToGitLab 统一权限 -> GitLab Access Level
func mapRoleToGitLab(unifiedRole int) int {
	switch unifiedRole {
	case 1:
		return 20 // Reporter
	case 2:
		return 30 // Developer
	case 3:
		return 40 // Maintainer
	case 4:
		return 50 // Owner
	default:
		return 20
	}
}

// mapRoleToGitea 统一权限 -> Gitea Permission 字符串
func mapRoleToGitea(unifiedRole int) string {
	switch unifiedRole {
	case 1:
		return "read"
	case 2:
		return "write"
	case 3:
		return "admin"
	case 4:
		return "admin" // Gitea no distinct owner role for collabs
	default:
		return "read"
	}
}

// @Summary      获取代码仓库成员列表
// @Description  获取代码仓库成员列表 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取代码仓库成员列表 响应结果"
// @Router       /code/getRepoMembers [get]
// @Security     Bearer
func getRepoMembers(c *gin.Context) {
	serverId, _ := strconv.Atoi(c.Query("serverId"))
	repoId, _ := strconv.Atoi(c.Query("repoId"))
	fullName := c.Query("fullName")

	if serverId == 0 || (repoId == 0 && fullName == "") {
		common.ReqBadFailWithMessage("缺少必要的参数(serverId, repoId, fullName)", c)
		return
	}

	server, err := models.GetCodeGitServerById(serverId)
	if err != nil {
		common.FailWithMessage("Git 实例配置不存在", c)
		return
	}

	members := make([]UnifiedMember, 0)

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		opt := &gitlab.ListProjectMembersOptions{
			ListOptions: gitlab.ListOptions{PerPage: 100},
		}
		gitlabMembers, _, err := client.ProjectMembers.ListAllProjectMembers(repoId, opt)
		if err == nil {
			for _, m := range gitlabMembers {
				level := 1
				if m.AccessLevel >= 50 {
					level = 4
				} else if m.AccessLevel >= 40 {
					level = 3
				} else if m.AccessLevel >= 30 {
					level = 2
				}
				members = append(members, UnifiedMember{
					Username:    m.Username,
					AvatarURL:   m.AvatarURL,
					AccessLevel: level,
				})
			}
		}
	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		parts := strings.Split(fullName, "/")
		if len(parts) == 2 {
			owner, repoName := parts[0], parts[1]

			opt := gitea.ListCollaboratorsOptions{
				ListOptions: gitea.ListOptions{PageSize: 100},
			}
			collabs, _, err := client.ListCollaborators(owner, repoName, opt)
			if err == nil {
				for _, m := range collabs {
					level := 1
					perm, _, err := client.CollaboratorPermission(owner, repoName, m.UserName)
					if err == nil && perm != nil {
						if perm.Permission == gitea.AccessModeOwner {
							level = 4
						} else if perm.Permission == gitea.AccessModeAdmin {
							level = 3
						} else if perm.Permission == gitea.AccessModeWrite {
							level = 2
						}
					}

					members = append(members, UnifiedMember{
						Username:    m.UserName,
						AvatarURL:   m.AvatarURL,
						AccessLevel: level,
					})
				}
			}
		}
	}

	common.OkWithData(members, c)
}

// @Summary      添加/更新仓库成员权限
// @Description  添加/更新仓库成员权限 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "添加/更新仓库成员权限 响应结果"
// @Router       /code/addRepoMember [post]
// @Security     Bearer
func addRepoMember(c *gin.Context) {
	var req MemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage("参数解析错误", c)
		return
	}

	server, err := models.GetCodeGitServerById(req.ServerID)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		// 1. 先通过 Username 查出 GitLab 内部的 User ID
		opt := &gitlab.ListUsersOptions{Username: gitlab.Ptr(req.Username)}
		users, _, err := client.Users.ListUsers(opt)
		if err != nil || len(users) == 0 {
			common.FailWithMessage("在GitLab中未找到该用户", c)
			return
		}
		gitLabUserId := users[0].ID

		// 2. 执行添加成员
		gitLabLevel := gitlab.AccessLevelValue(mapRoleToGitLab(req.AccessLevel))
		addOpt := &gitlab.AddProjectMemberOptions{
			UserID:      gitlab.Ptr(gitLabUserId),
			AccessLevel: gitlab.Ptr(gitLabLevel),
		}
		_, resp, err := client.ProjectMembers.AddProjectMember(req.RepoID, addOpt)

		// GitLab 如果用户已存在会返回 409，此时我们改为调用 PUT 接口更新权限
		if resp != nil && resp.StatusCode == http.StatusConflict {
			editOpt := &gitlab.EditProjectMemberOptions{
				AccessLevel: gitlab.Ptr(gitLabLevel),
			}
			_, _, _ = client.ProjectMembers.EditProjectMember(req.RepoID, gitLabUserId, editOpt)
		} else if err != nil {
			common.FailWithMessage("请求GitLab API失败", c)
			return
		}

	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		parts := strings.Split(req.FullName, "/")
		if len(parts) != 2 {
			common.FailWithMessage("无效的仓库名称格式", c)
			return
		}
		owner, repoName := parts[0], parts[1]
		giteaLevel := gitea.AccessMode(mapRoleToGitea(req.AccessLevel))

		// Gitea的PUT如果用户已存在可能不会更新权限，先执行一次安全删除
		_, _ = client.DeleteCollaborator(owner, repoName, req.Username)
		opt := gitea.AddCollaboratorOption{Permission: &giteaLevel}
		_, err = client.AddCollaborator(owner, repoName, req.Username, opt)
		if err != nil {
			common.FailWithMessage("添加/更新 Gitea 成员失败", c)
			return
		}
	}

	common.OkWithMessage("成员权限配置成功", c)
}

// @Summary      移除仓库成员
// @Description  移除仓库成员 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "移除仓库成员 响应结果"
// @Router       /code/removeRepoMember [delete]
// @Security     Bearer
func removeRepoMember(c *gin.Context) {
	var req MemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage("参数解析错误", c)
		return
	}

	server, err := models.GetCodeGitServerById(req.ServerID)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		opt := &gitlab.ListUsersOptions{Username: gitlab.Ptr(req.Username)}
		users, _, err := client.Users.ListUsers(opt)
		if err != nil || len(users) == 0 {
			common.OkWithMessage("成员已移除", c) // 找不到用户视同已移除
			return
		}

		_, err = client.ProjectMembers.DeleteProjectMember(req.RepoID, users[0].ID, nil)
		if err != nil {
			// skip
		}

	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}
		parts := strings.Split(req.FullName, "/")
		if len(parts) == 2 {
			owner, repoName := parts[0], parts[1]
			_, err = client.DeleteCollaborator(owner, repoName, req.Username)
		}
	}

	common.OkWithMessage("移除成员成功", c)
}

// 统一的返回分支结构体
type GitBranch struct {
	Name string `json:"name"`
}

// @Summary      获取代码仓库分支列表
// @Description  获取代码仓库分支列表 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取代码仓库分支列表 响应结果"
// @Router       /code/getRepoBranches [get]
// @Security     Bearer
func fetchBranchesForServer(server models.CodeGitServer, repoId int, fullName string) ([]GitBranch, error) {
	branches := make([]GitBranch, 0)
	if server.Platform == "gitlab" {
		client, err := getGitLabClient(&server)
		if err != nil {
			return nil, err
		}
		var pid interface{} = repoId
		if repoId == 0 && fullName != "" {
			pid = fullName
		}
		gitlabBranches, _, err := client.Branches.ListBranches(pid, &gitlab.ListBranchesOptions{})
		if err != nil {
			return nil, err
		}
		for _, b := range gitlabBranches {
			branches = append(branches, GitBranch{Name: b.Name})
		}
		return branches, nil
	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(&server)
		if err != nil {
			return nil, err
		}
		parts := strings.Split(fullName, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid fullName format")
		}
		owner, repoName := parts[0], parts[1]
		giteaBranches, _, err := client.ListRepoBranches(owner, repoName, gitea.ListRepoBranchesOptions{})
		if err != nil {
			return nil, err
		}
		for _, b := range giteaBranches {
			branches = append(branches, GitBranch{Name: b.Name})
		}
		return branches, nil
	}
	return nil, fmt.Errorf("unsupported platform")
}

func getRepoBranches(c *gin.Context) {
	serverId, _ := strconv.Atoi(c.Query("serverId"))
	repoId, _ := strconv.Atoi(c.Query("repoId"))
	fullName := c.Query("fullName")

	if repoId == 0 && fullName == "" {
		common.OkWithData([]GitBranch{}, c)
		return
	}

	if serverId > 0 {
		server, err := models.GetCodeGitServerById(serverId)
		if err == nil {
			if branches, err := fetchBranchesForServer(*server, repoId, fullName); err == nil && len(branches) > 0 {
				common.OkWithData(branches, c)
				return
			}
		}
	}

	// 如果未指定 serverId 或特定 server 获取失败，遍历系统包含的所有 Git 实例匹配
	var servers []models.CodeGitServer
	_ = models.Db.Find(&servers).Error
	for _, s := range servers {
		if branches, err := fetchBranchesForServer(s, repoId, fullName); err == nil && len(branches) > 0 {
			common.OkWithData(branches, c)
			return
		}
	}

	// 保底逻辑：返回默认主流分支
	fallback := []GitBranch{
		{Name: "main"},
		{Name: "master"},
		{Name: "develop"},
	}
	common.OkWithData(fallback, c)
}

// 统一的命名空间结构，返给前端
type GitNamespace struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"` // GitLab 的 path 或 Gitea 的 org name
	Kind string `json:"kind"` // "user" (个人) 或 "group"/"org" (组织/群组)
}

// 接收新建命名空间的请求
type CreateNamespaceReq struct {
	ServerID   int    `json:"serverId" binding:"required"`
	Name       string `json:"name" binding:"required"`
	Path       string `json:"path" binding:"required"`
	OldPath    string `json:"oldPath"`    // 用于更新时识别原路径
	Visibility string `json:"visibility"` // private, internal, public
}

// @Summary      获取Git系统命名空间/Group列表
// @Description  获取Git系统命名空间/Group列表 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取Git系统命名空间/Group列表 响应结果"
// @Router       /code/getGitNamespaces [get]
// @Security     Bearer
func getGitNamespaces(c *gin.Context) {
	serverId, _ := strconv.Atoi(c.Query("serverId"))
	if serverId == 0 {
		common.OkWithDetailed(gin.H{"items": make([]GitNamespace, 0), "total": 0}, "请先选择所属Git实例", c)
		return
	}

	server, err := models.GetCodeGitServerById(serverId)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	results := make([]GitNamespace, 0)

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		opt := &gitlab.ListNamespacesOptions{
			ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1},
		}
		for {
			namespaces, resp, err := client.Namespaces.ListNamespaces(opt)
			if err != nil {
				break
			}
			for _, ns := range namespaces {
				results = append(results, GitNamespace{ID: ns.ID, Name: ns.Name, Path: ns.Path, Kind: ns.Kind})
			}
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		user, _, err := client.GetMyUserInfo()
		if err == nil {
			results = append(results, GitNamespace{ID: int(user.ID), Name: user.UserName, Path: user.UserName, Kind: "user"})
		}

		opt := gitea.ListOrgsOptions{
			ListOptions: gitea.ListOptions{PageSize: 100, Page: 1},
		}
		for {
			orgs, _, err := client.ListMyOrgs(opt)
			if err != nil || len(orgs) == 0 {
				break
			}
			for _, org := range orgs {
				results = append(results, GitNamespace{ID: int(org.ID), Name: org.UserName, Path: org.UserName, Kind: "org"})
			}
			if len(orgs) < 100 {
				break
			}
			opt.Page++
		}
	}

	nameFilter := strings.ToLower(c.Query("name"))
	kindFilter := strings.ToLower(c.Query("kind"))

	filteredResults := make([]GitNamespace, 0)
	for _, ns := range results {
		if nameFilter != "" && !strings.Contains(strings.ToLower(ns.Name), nameFilter) {
			continue
		}
		if kindFilter != "" {
			if kindFilter == "group" && ns.Kind != "group" && ns.Kind != "org" {
				continue
			}
			if kindFilter != "group" && strings.ToLower(ns.Kind) != kindFilter {
				continue
			}
		}
		filteredResults = append(filteredResults, ns)
	}

	common.OkWithData(filteredResults, c)
}

// @Summary      创建Git命名空间
// @Description  创建Git命名空间 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "创建Git命名空间 响应结果"
// @Router       /code/createGitNamespace [post]
// @Security     Bearer
func createGitNamespace(c *gin.Context) {
	var req CreateNamespaceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage("参数解析错误", c)
		return
	}

	server, err := models.GetCodeGitServerById(req.ServerID)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		visibility := gitlab.VisibilityValue(req.Visibility)
		if req.Visibility == "" {
			visibility = gitlab.PrivateVisibility
		}

		opt := &gitlab.CreateGroupOptions{
			Name:       gitlab.Ptr(req.Name),
			Path:       gitlab.Ptr(req.Path),
			Visibility: gitlab.Ptr(visibility),
		}
		_, _, err = client.Groups.CreateGroup(opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("GitLab 创建 Group 失败: %v", err), c)
			return
		}

	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		visibility := gitea.VisibleTypePrivate
		if req.Visibility == "public" {
			visibility = gitea.VisibleTypePublic
		}

		opt := gitea.CreateOrgOption{
			Name:       req.Path, // Gitea 使用 Path 作为组织唯一标识
			Visibility: visibility,
		}
		_, _, err = client.CreateOrg(opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("Gitea 创建 Organization 失败: %v", err), c)
			return
		}
	}

	common.OkWithMessage("命名空间创建成功", c)
}

// @Summary      更新Git命名空间
// @Description  更新Git命名空间 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "更新Git命名空间 响应结果"
// @Router       /code/updateGitNamespace [post]
// @Security     Bearer
func updateGitNamespace(c *gin.Context) {
	var req CreateNamespaceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage("参数解析错误", c)
		return
	}

	server, err := models.GetCodeGitServerById(req.ServerID)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		// Check if path is available if it changed
		if req.OldPath != "" && req.OldPath != req.Path {
			_, resp, err := client.Groups.GetGroup(req.Path, nil)
			if err == nil || resp.StatusCode != 404 {
				common.FailWithMessage("更新失败: 该命名空间路径已被占用", c)
				return
			}
		}

		visibility := gitlab.VisibilityValue(req.Visibility)
		if req.Visibility == "" {
			visibility = gitlab.PrivateVisibility
		}

		opt := &gitlab.UpdateGroupOptions{
			Name:       gitlab.Ptr(req.Name),
			Path:       gitlab.Ptr(req.Path),
			Visibility: gitlab.Ptr(visibility),
		}

		targetPath := req.Path
		if req.OldPath != "" {
			targetPath = req.OldPath
		}

		_, _, err = client.Groups.UpdateGroup(targetPath, opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("GitLab 更新 Group 失败: %v", err), c)
			return
		}

	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		// Check if path is available if it changed
		if req.OldPath != "" && req.OldPath != req.Path {
			_, resp, err := client.GetOrg(req.Path)
			if err == nil || resp.StatusCode != 404 {
				common.FailWithMessage("更新失败: 该命名空间路径已被占用", c)
				return
			}
		}

		visibility := gitea.VisibleTypePrivate
		if req.Visibility == "public" {
			visibility = gitea.VisibleTypePublic
		}

		opt := gitea.EditOrgOption{
			FullName:   req.Name,
			Visibility: visibility,
		}

		// Gitea 1.26+ doesn't support changing org name/path directly via API? Wait, does it?
		// EditOrgOption has Name, FullName... actually changing the path (username of org) is not well supported in Gitea's standard API.
		// Wait! Gitea does not allow changing an organization's name/path via API `EditOrg`. The `EditOrgOption` only has `FullName` (which is display name), `Description`, `Website`, `Location`, `Visibility`.
		// It doesn't have `Name` (the path). If the user tries to change the path, Gitea API will ignore it or fail.
		// So if path changed on Gitea, we must tell the user it's not supported.
		if req.OldPath != "" && req.OldPath != req.Path {
			common.FailWithMessage("Gitea 不支持通过 API 修改命名空间路径 (仅支持修改显示名称)", c)
			return
		}

		targetPath := req.Path
		if req.OldPath != "" {
			targetPath = req.OldPath
		}

		_, err = client.EditOrg(targetPath, opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("Gitea 更新 Organization 失败: %v", err), c)
			return
		}
	}

	common.OkWithMessage("命名空间更新成功", c)
}

// ------------------- 用户结构定义 -------------------
type GitUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	State    string `json:"state"`
}

type CreateGitUserReq struct {
	ServerID int    `json:"serverId" binding:"required"`
	Username string `json:"username" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UpdateGitUserReq struct {
	ServerID int    `json:"serverId" binding:"required"`
	Username string `json:"username" binding:"required"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	State    string `json:"state"`
}

// @Summary      获取Git端用户列表
// @Description  获取Git端用户列表 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取Git端用户列表 响应结果"
// @Router       /code/getGitUsers [get]
// @Security     Bearer
func getGitUsers(c *gin.Context) {
	serverId, _ := strconv.Atoi(c.Query("serverId"))
	if serverId == 0 {
		common.OkWithDetailed(gin.H{"items": make([]GitUser, 0), "total": 0}, "请先选择所属Git实例", c)
		return
	}

	server, err := models.GetCodeGitServerById(serverId)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	results := make([]GitUser, 0)

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		opt := &gitlab.ListUsersOptions{
			ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1},
		}
		for {
			users, resp, err := client.Users.ListUsers(opt)
			if err != nil {
				break
			}
			for _, u := range users {
				results = append(results, GitUser{ID: u.ID, Username: u.Username, Name: u.Name, Email: u.Email, State: u.State})
			}
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		users, _, err := client.AdminListUsers(gitea.AdminListUsersOptions{})
		if err != nil {
			// fallback if AdminListUsers doesn't exist or returns error
			users, _, _ = client.AdminListUsers(gitea.AdminListUsersOptions{}) // retry or ignore
		}

		for _, u := range users {
			state := "blocked"
			if u.ProhibitLogin {
				state = "banned"
			} else if u.IsActive {
				state = "active"
			}
			results = append(results, GitUser{ID: int(u.ID), Username: u.UserName, Name: u.FullName, Email: u.Email, State: state})
		}
	}

	usernameFilter := strings.ToLower(c.Query("username"))
	nameFilter := strings.ToLower(c.Query("name"))
	emailFilter := strings.ToLower(c.Query("email"))
	stateFilter := strings.ToLower(c.Query("state"))

	filteredResults := make([]GitUser, 0)
	for _, u := range results {
		if usernameFilter != "" && !strings.Contains(strings.ToLower(u.Username), usernameFilter) {
			continue
		}
		if nameFilter != "" && !strings.Contains(strings.ToLower(u.Name), nameFilter) {
			continue
		}
		if emailFilter != "" && !strings.Contains(strings.ToLower(u.Email), emailFilter) {
			continue
		}
		if stateFilter != "" && strings.ToLower(u.State) != stateFilter {
			continue
		}
		filteredResults = append(filteredResults, u)
	}

	common.OkWithData(filteredResults, c)
}

// @Summary      创建Git端用户
// @Description  创建Git端用户 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "创建Git端用户 响应结果"
// @Router       /code/createGitUser [post]
// @Security     Bearer
func createGitUser(c *gin.Context) {
	var req CreateGitUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage("参数解析错误", c)
		return
	}

	server, err := models.GetCodeGitServerById(req.ServerID)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		opt := &gitlab.CreateUserOptions{
			Username:         gitlab.Ptr(req.Username),
			Name:             gitlab.Ptr(req.Name),
			Email:            gitlab.Ptr(req.Email),
			Password:         gitlab.Ptr(req.Password),
			SkipConfirmation: gitlab.Ptr(true),
		}
		_, _, err = client.Users.CreateUser(opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("GitLab 创建用户失败: %v", err), c)
			return
		}

	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		opt := gitea.CreateUserOption{
			LoginName:  req.Username,
			Username:   req.Username,
			FullName:   req.Name,
			Email:      req.Email,
			Password:   req.Password,
			SendNotify: false,
		}
		_, _, err = client.AdminCreateUser(opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("Gitea 创建用户失败: %v", err), c)
			return
		}
	}

	common.OkWithMessage("用户创建成功", c)
}

// @Summary      更新Git端用户
// @Description  更新Git端用户 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "更新Git端用户 响应结果"
// @Router       /code/updateGitUser [post]
// @Security     Bearer
func updateGitUser(c *gin.Context) {
	var req UpdateGitUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ReqBadFailWithMessage("参数解析错误", c)
		return
	}

	server, err := models.GetCodeGitServerById(req.ServerID)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		optSearch := &gitlab.ListUsersOptions{Username: gitlab.Ptr(req.Username)}
		users, _, err := client.Users.ListUsers(optSearch)
		if err != nil || len(users) == 0 {
			common.FailWithMessage("在GitLab中未找到该用户", c)
			return
		}

		opt := &gitlab.ModifyUserOptions{
			Name:  gitlab.Ptr(req.Name),
			Email: gitlab.Ptr(req.Email),
		}
		if req.Password != "" {
			opt.Password = gitlab.Ptr(req.Password)
		}
		_, _, err = client.Users.ModifyUser(users[0].ID, opt)
		if req.State != "" && req.State != users[0].State {
			var stateErr error
			if req.State == "blocked" || req.State == "inactive" {
				if users[0].State == "banned" {
					client.Users.UnbanUser(users[0].ID)
				}
				stateErr = client.Users.BlockUser(users[0].ID)
			} else if req.State == "banned" {
				if users[0].State == "blocked" {
					client.Users.UnblockUser(users[0].ID)
				}
				stateErr = client.Users.BanUser(users[0].ID)
			} else if req.State == "active" {
				if users[0].State == "banned" {
					stateErr = client.Users.UnbanUser(users[0].ID)
				} else {
					stateErr = client.Users.UnblockUser(users[0].ID)
				}
			}
			if stateErr != nil {
				common.FailWithMessage(fmt.Sprintf("GitLab 更新用户状态失败: %v", stateErr), c)
				return
			}
		}
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("GitLab 更新用户失败: %v", err), c)
			return
		}

	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		opt := gitea.EditUserOption{
			LoginName: req.Username,
			FullName:  &req.Name,
			Email:     &req.Email,
		}
		if req.Password != "" {
			opt.Password = req.Password
		}
		if req.State == "blocked" || req.State == "inactive" {
			b := false
			opt.Active = &b
			p := false
			opt.ProhibitLogin = &p
		} else if req.State == "active" {
			b := true
			opt.Active = &b
			p := false
			opt.ProhibitLogin = &p
		} else if req.State == "banned" {
			b := false
			opt.Active = &b
			p := true
			opt.ProhibitLogin = &p
		}
		_, err = client.AdminEditUser(req.Username, opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("Gitea 更新用户失败: %v", err), c)
			return
		}
	}

	common.OkWithMessage("用户更新成功", c)
}

func createRemoteRepo(server *models.CodeGitServer, repo *models.CodeGitRepo) (int, string, string, error) {
	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			return 0, "", "", fmt.Errorf("初始化 GitLab 客户端失败: %v", err)
		}

		visibility := gitlab.VisibilityValue(repo.Visibility)
		opt := &gitlab.CreateProjectOptions{
			Name:        gitlab.Ptr(repo.Name),
			Description: gitlab.Ptr(repo.Description),
			Visibility:  gitlab.Ptr(visibility),
		}
		if repo.NamespaceID > 0 {
			opt.NamespaceID = gitlab.Ptr(repo.NamespaceID)
		} else if repo.NamespacePath != "" {
			ns, _, err := client.Namespaces.GetNamespace(repo.NamespacePath)
			if err == nil && ns != nil {
				opt.NamespaceID = gitlab.Ptr(ns.ID)
			}
		}

		project, _, err := client.Projects.CreateProject(opt)
		if err != nil {
			return 0, "", "", fmt.Errorf("GitLab API 创建仓库失败: %v", err)
		}
		return project.ID, project.WebURL, project.PathWithNamespace, nil

	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			return 0, "", "", fmt.Errorf("初始化 Gitea 客户端失败: %v", err)
		}

		private := repo.Visibility == "private"
		opt := gitea.CreateRepoOption{
			Name:        repo.Name,
			Description: repo.Description,
			Private:     private,
		}

		var project *gitea.Repository
		if repo.NamespacePath != "" && repo.NamespacePath != server.Username {
			project, _, err = client.CreateOrgRepo(repo.NamespacePath, opt)
		} else {
			project, _, err = client.CreateRepo(opt)
		}

		if err != nil {
			return 0, "", "", fmt.Errorf("Gitea API 创建仓库失败: %v", err)
		}
		return int(project.ID), project.HTMLURL, project.FullName, nil
	}

	return 0, "", "", fmt.Errorf("不支持的平台")
}

// updateRemoteRepo 同步更新远程 Git 仓库信息
func updateRemoteRepo(server *models.CodeGitServer, oldRepo *models.CodeGitRepo, newRepo *models.CodeGitRepo) error {
	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			return fmt.Errorf("初始化 GitLab 客户端失败: %v", err)
		}

		visibility := gitlab.VisibilityValue(newRepo.Visibility)
		opt := &gitlab.EditProjectOptions{
			Name:        gitlab.Ptr(newRepo.Name),
			Description: gitlab.Ptr(newRepo.Description),
			Visibility:  gitlab.Ptr(visibility),
		}

		_, _, err = client.Projects.EditProject(oldRepo.ProjectID, opt)
		if err != nil {
			return fmt.Errorf("GitLab API 更新仓库失败: %v", err)
		}
		return nil

	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			return fmt.Errorf("初始化 Gitea 客户端失败: %v", err)
		}

		private := newRepo.Visibility == "private"
		opt := gitea.EditRepoOption{
			Name:        &newRepo.Name,
			Description: &newRepo.Description,
			Private:     &private,
		}

		parts := strings.Split(oldRepo.FullName, "/")
		if len(parts) != 2 {
			return fmt.Errorf("无效的仓库名称格式: %s", oldRepo.FullName)
		}
		owner, repoName := parts[0], parts[1]

		_, _, err = client.EditRepo(owner, repoName, opt)
		if err != nil {
			return fmt.Errorf("Gitea API 更新仓库失败: %v", err)
		}
		return nil
	}

	return fmt.Errorf("不支持的平台")
}

// ------------------- [API] 合并请求管理 -------------------

type MergeRequest struct {
	ID           int    `json:"id"`
	IID          int    `json:"iid"` // GitLab MR IID / Gitea PR Index
	Title        string `json:"title"`
	SourceBranch string `json:"sourceBranch"`
	TargetBranch string `json:"targetBranch"`
	Author       string `json:"author"`
	Status       string `json:"status"` // pending, merged, closed
	Time         string `json:"time"`
}

type DoMergeRequestReq struct {
	ServerID           int    `json:"serverId" binding:"required"`
	RepoID             int    `json:"repoId"`
	FullName           string `json:"fullName"`
	MRID               int    `json:"mrId" binding:"required"`
	Strategy           string `json:"strategy"`
	DeleteSourceBranch bool   `json:"deleteSourceBranch"`
	Message            string `json:"message"`
}

type CreateMergeRequestReq struct {
	ServerID     int    `json:"serverId" binding:"required"`
	RepoID       int    `json:"repoId"`
	FullName     string `json:"fullName"`
	Title        string `json:"title" binding:"required"`
	Description  string `json:"description"`
	SourceBranch string `json:"sourceBranch" binding:"required"`
	TargetBranch string `json:"targetBranch" binding:"required"`
}

// @Summary      获取代码合并请求 (MR/PR) 列表
// @Description  获取代码合并请求 (MR/PR) 列表 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "获取代码合并请求 (MR/PR) 列表 响应结果"
// @Router       /code/getMergeRequests [get]
// @Security     Bearer
func getMergeRequests(c *gin.Context) {
	serverId, _ := strconv.Atoi(c.Query("serverId"))
	repoId, _ := strconv.Atoi(c.Query("repoId"))
	fullName := c.Query("fullName")
	statusFilter := c.Query("status")

	if serverId == 0 || (repoId == 0 && fullName == "") {
		common.OkWithDetailed(gin.H{"items": make([]MergeRequest, 0), "total": 0}, "请先选择所属Git实例和代码仓库", c)
		return
	}

	server, err := models.GetCodeGitServerById(serverId)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	results := make([]MergeRequest, 0)

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		opt := &gitlab.ListProjectMergeRequestsOptions{
			ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1},
		}
		if statusFilter != "" {
			if statusFilter == "pending" {
				opt.State = gitlab.Ptr("opened")
			} else if statusFilter == "merged" {
				opt.State = gitlab.Ptr("merged")
			} else if statusFilter == "closed" {
				opt.State = gitlab.Ptr("closed")
			}
		}

		for {
			mrs, resp, err := client.MergeRequests.ListProjectMergeRequests(repoId, opt)
			if err != nil {
				break
			}
			for _, mr := range mrs {
				status := "pending"
				if mr.State == "merged" {
					status = "merged"
				} else if mr.State == "closed" {
					status = "closed"
				}

				results = append(results, MergeRequest{
					ID:           mr.ID,
					IID:          mr.IID,
					Title:        mr.Title,
					SourceBranch: mr.SourceBranch,
					TargetBranch: mr.TargetBranch,
					Author:       mr.Author.Name,
					Status:       status,
					Time:         mr.UpdatedAt.Format("2006-01-02 15:04:05"),
				})
			}
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}

	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		parts := strings.Split(fullName, "/")
		if len(parts) != 2 {
			common.FailWithMessage("Gitea 仓库名称格式不正确", c)
			return
		}
		owner, repoName := parts[0], parts[1]

		giteaState := gitea.StateAll
		if statusFilter == "pending" {
			giteaState = gitea.StateOpen
		} else if statusFilter == "closed" || statusFilter == "merged" {
			giteaState = gitea.StateClosed
		}

		opt := gitea.ListPullRequestsOptions{
			State:       giteaState,
			ListOptions: gitea.ListOptions{Page: 1, PageSize: 100},
		}

		for {
			prs, resp, err := client.ListRepoPullRequests(owner, repoName, opt)
			if err != nil {
				break
			}
			for _, pr := range prs {
				status := "pending"
				if pr.State == "closed" {
					if pr.HasMerged {
						status = "merged"
					} else {
						status = "closed"
					}
				}

				if statusFilter == "merged" && status != "merged" {
					continue
				}
				if statusFilter == "closed" && status != "closed" {
					continue
				}

				authorName := pr.Poster.UserName
				if pr.Poster.FullName != "" {
					authorName = pr.Poster.FullName
				}

				results = append(results, MergeRequest{
					ID:           int(pr.ID),
					IID:          int(pr.Index),
					Title:        pr.Title,
					SourceBranch: pr.Head.Name,
					TargetBranch: pr.Base.Name,
					Author:       authorName,
					Status:       status,
					Time:         pr.Updated.Format("2006-01-02 15:04:05"),
				})
			}

			if resp == nil || len(prs) == 0 || resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
	}

	common.OkWithDetailed(gin.H{"items": results, "total": len(results)}, "获取成功", c)
}

// @Summary      执行合并代码请求
// @Description  执行合并代码请求 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "执行合并代码请求 响应结果"
// @Router       /code/mergeMergeRequest [post]
// @Security     Bearer
func mergeMergeRequest(c *gin.Context) {
	var req DoMergeRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}

	server, err := models.GetCodeGitServerById(req.ServerID)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		opt := &gitlab.AcceptMergeRequestOptions{
			ShouldRemoveSourceBranch: gitlab.Ptr(req.DeleteSourceBranch),
		}
		if req.Strategy == "squash" {
			opt.Squash = gitlab.Ptr(true)
		}
		if req.Message != "" {
			opt.MergeCommitMessage = gitlab.Ptr(req.Message)
		}

		_, _, err = client.MergeRequests.AcceptMergeRequest(req.RepoID, req.MRID, opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("GitLab 合并失败: %v", err), c)
			return
		}

	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		parts := strings.Split(req.FullName, "/")
		if len(parts) != 2 {
			common.FailWithMessage("Gitea 仓库名称格式不正确", c)
			return
		}
		owner, repoName := parts[0], parts[1]

		style := gitea.MergeStyleMerge
		if req.Strategy == "squash" {
			style = gitea.MergeStyleSquash
		}

		opt := gitea.MergePullRequestOption{
			Style:   style,
			Message: req.Message,
		}

		_, _, err = client.MergePullRequest(owner, repoName, int64(req.MRID), opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("Gitea 合并失败: %v", err), c)
			return
		}

		if req.DeleteSourceBranch {
			pr, _, err := client.GetPullRequest(owner, repoName, int64(req.MRID))
			if err == nil && pr.Head != nil {
				client.DeleteRepoBranch(owner, repoName, pr.Head.Name)
			}
		}

	} else {
		common.FailWithMessage("不支持的平台", c)
		return
	}

	common.OkWithMessage("合并成功", c)
}

// @Summary      关闭代码合并请求
// @Description  关闭代码合并请求 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "关闭代码合并请求 响应结果"
// @Router       /code/closeMergeRequest [post]
// @Security     Bearer
func closeMergeRequest(c *gin.Context) {
	var req DoMergeRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}

	server, err := models.GetCodeGitServerById(req.ServerID)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		opt := &gitlab.UpdateMergeRequestOptions{
			StateEvent: gitlab.Ptr("close"),
		}
		_, _, err = client.MergeRequests.UpdateMergeRequest(req.RepoID, req.MRID, opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("GitLab 关闭合并请求失败: %v", err), c)
			return
		}

	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		parts := strings.Split(req.FullName, "/")
		if len(parts) != 2 {
			common.FailWithMessage("Gitea 仓库名称格式不正确", c)
			return
		}
		owner, repoName := parts[0], parts[1]

		state := gitea.StateClosed
		opt := gitea.EditPullRequestOption{
			State: &state,
		}

		_, _, err = client.EditPullRequest(owner, repoName, int64(req.MRID), opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("Gitea 关闭合并请求失败: %v", err), c)
			return
		}
	} else {
		common.FailWithMessage("不支持的平台", c)
		return
	}

	common.OkWithMessage("合并请求已拒绝/关闭", c)
}

// @Summary      创建代码合并请求
// @Description  创建代码合并请求 接口
// @Tags         code-git
// @Accept       json
// @Produce      json
// @Success      200 {object} common.BaseResp "创建代码合并请求 响应结果"
// @Router       /code/createMergeRequest [post]
// @Security     Bearer
func createMergeRequest(c *gin.Context) {
	var req CreateMergeRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.FailWithMessage(err.Error(), c)
		return
	}

	server, err := models.GetCodeGitServerById(req.ServerID)
	if err != nil {
		common.FailWithMessage("Git 实例不存在", c)
		return
	}

	if server.Platform == "gitlab" {
		client, err := getGitLabClient(server)
		if err != nil {
			common.FailWithMessage("初始化 GitLab 客户端失败", c)
			return
		}

		opt := &gitlab.CreateMergeRequestOptions{
			Title:        gitlab.Ptr(req.Title),
			Description:  gitlab.Ptr(req.Description),
			SourceBranch: gitlab.Ptr(req.SourceBranch),
			TargetBranch: gitlab.Ptr(req.TargetBranch),
		}

		_, _, err = client.MergeRequests.CreateMergeRequest(req.RepoID, opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("GitLab 创建合并请求失败: %v", err), c)
			return
		}

	} else if server.Platform == "gitea" {
		client, err := getGiteaClient(server)
		if err != nil {
			common.FailWithMessage("初始化 Gitea 客户端失败", c)
			return
		}

		parts := strings.Split(req.FullName, "/")
		if len(parts) != 2 {
			common.FailWithMessage("Gitea 仓库名称格式不正确", c)
			return
		}
		owner, repoName := parts[0], parts[1]

		opt := gitea.CreatePullRequestOption{
			Title: req.Title,
			Body:  req.Description,
			Head:  req.SourceBranch,
			Base:  req.TargetBranch,
		}

		_, _, err = client.CreatePullRequest(owner, repoName, opt)
		if err != nil {
			common.FailWithMessage(fmt.Sprintf("Gitea 创建合并请求失败: %v", err), c)
			return
		}
	} else {
		common.FailWithMessage("不支持的平台", c)
		return
	}

	common.OkWithMessage("合并请求已创建", c)
}
