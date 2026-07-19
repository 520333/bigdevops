package models

import (
	"fmt"
)

type CodeGitRepo struct {
	ID        uint `json:"id"`
	ServerID  uint `json:"serverId"`
	ProjectID int  `json:"projectId"`
	UserID    uint `json:"userId"`

	Name          string `json:"name"`
	FullName      string `json:"fullName"`
	Description   string `json:"description"`
	DefaultBranch string `json:"defaultBranch"`
	Visibility    string `json:"visibility"`

	CloneUrlHttp string `json:"cloneUrlHttp"`
	CloneUrlSsh  string `json:"cloneUrlSsh"`
	WebUrl       string `json:"webUrl"`

	SyncStatus string `json:"syncStatus"`

	NamespaceID   int    `json:"namespaceId"`
	NamespacePath string `json:"namespacePath"`

	ExternalLabelsFront string `json:"externalLabelsFront"`
	ServerName          string `json:"serverName"`
	Key                 string `json:"key"`
	CreateUserName      string `json:"createUserName"`
	OwnerName           string `json:"ownerName"`
}

// FillFrontAllData 填充前端需要的虚拟字段
func (obj *CodeGitRepo) FillFrontAllData() {

	serverInfo, _ := GetCodeGitServerById(int(obj.ServerID))
	if serverInfo != nil {
		obj.ServerName = serverInfo.Name
	} else {
		obj.ServerName = "未知服务"
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)
}
