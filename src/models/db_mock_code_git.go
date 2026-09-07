package models

import (
	"bigdevops/src/config"
)

func mockCodeGitData(sc *config.ServerConfig, adminUser *SystemUser) {
	// prometheus 实例池
	//name := []string{"集团自建 GitLab", "研发二部 Gitea", "pre", "prod"}
	//endpoints := []string{"https://gitlab.mycorp.com", "http://192.168.1.100:3000"}
	//platforms := []string{"gitlab", "gitea"}

	s := CodeGitServer{
		Name:       "开发机测试gitea",
		Platform:   "gitea",
		Endpoint:   "http://192.168.50.100:13000",
		UserID:     1,
		Token:      "23db28b76480289030b659e4d89346c4abc776c4",
		SkipVerify: true,
	}
	_ = s.CreateOne()

	s2 := CodeGitServer{
		Name:       "公司内网-生产gitlab",
		Platform:   "gitlab",
		Endpoint:   "http://192.168.1.160", // 替换为安全的占位 IP
		UserID:     1,
		Token:      "glpat-rrsKMShuqvAsnkRaFj9J", // 替换为安全的占位 Token
		SkipVerify: true,
	}
	_ = s2.CreateOne()

	sc.Logger.Info("代码仓库数据 Mock 数据注入成功")

}
