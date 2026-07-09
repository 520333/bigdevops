package cron

import (
	"bigdevops/src/common"
	"bigdevops/src/models"
	"encoding/json"
	"fmt"
	"time"

	"github.com/prometheus/alertmanager/template"
)

var (
	feiShuCardJsonMsg = `{
	"msg_type": "interactive",
	"card": {
		"schema": "2.0",
		"config": {
			"update_multi": true
		},
		"body": {
			"direction": "vertical",
			"elements": [{
					"tag": "img",
					"img_key": "img_v3_0213c_3f541aee-e41b-4640-b078-3f31c81ab25g",
					"preview": false,
					"scale_type": "fit_horizontal",
					"corner_radius": "8px",
					"margin": "0px 0px 0px 0px"
				},
				{
					"tag": "markdown",
					"content": "**<font color='red'>告警级别：</font>** 严重\n**<font color='grey'>触发时间：</font>** 2024-05-20 14:30:22\n**<font color='grey'>告警描述：</font>** 服务器CPU使用率持续10分钟超过95%，可能影响服务稳定性，请及时处理。",
					"text_size": "normal",
					"margin": "0px 0px 0px 0px",
					"element_id": "MYCvpK5nMTC6oWd6x_AC"
				},
				{
					"tag": "column_set",
					"flex_mode": "stretch",
					"horizontal_spacing": "8px",
					"horizontal_align": "left",
					"columns": [{
							"tag": "column",
							"width": "auto",
							"elements": [{
								"tag": "button",
								"text": {
									"tag": "plain_text",
									"content": "查看prom数据"
								},
								"type": "primary_filled",
								"width": "default",
								"behaviors": [{
									"type": "open_url",
									"default_url": "http://192.168.50.200:9090",
									"pc_url": "",
									"ios_url": "",
									"android_url": ""
								}],
								"margin": "4px 0px 4px 0px",
								"element_id": "UB9dGyQ2OoCCMgxi56Fn"
							}],
							"vertical_spacing": "8px",
							"horizontal_align": "left",
							"vertical_align": "top"
						},
						{
							"tag": "column",
							"width": "auto",
							"elements": [{
								"tag": "button",
								"text": {
									"tag": "plain_text",
									"content": "确认"
								},
								"type": "default",
								"width": "default",
								"confirm": {
									"title": {
										"tag": "plain_text",
										"content": "确认删除吗"
									},
									"text": {
										"tag": "plain_text",
										"content": "删除保护"
									}
								},
								"behaviors": [{
									"type": "callback",
									"value": {
										"action": "ignore_alert"
									}
								}],
								"margin": "4px 0px 4px 0px",
								"element_id": "RutyrtDq4bQvCcB4MpMT"
							}],
							"vertical_spacing": "8px",
							"horizontal_align": "left",
							"vertical_align": "top"
						}
					],
					"margin": "0px 0px 0px 0px"
				}
			]
		},
		"header": {
			"title": {
				"tag": "plain_text",
				"content": "系统异常告警"
			},
			"subtitle": {
				"tag": "plain_text",
				"content": ""
			},
			"text_tag_list": [{
				"tag": "text_tag",
				"text": {
					"tag": "plain_text",
					"content": "紧急"
				},
				"color": "red"
			}],
			"template": "red",
			"icon": {
				"tag": "standard_icon",
				"token": "alert-circle_outlined"
			},
			"padding": "12px 8px 12px 8px"
		}
	}
}`
	// 群和私聊公用的json
	feiShuCardContent = `{
    "schema": "2.0",
    "config": {
        "update_multi": true,
        "style": {
            "text_size": {
                "normal_v2": {
                    "default": "normal",
                    "pc": "normal",
                    "mobile": "heading"
                }
            }
        }
    },
    "body": {
        "direction": "vertical",
        "horizontal_spacing": "8px",
        "vertical_spacing": "8px",
        "horizontal_align": "left",
        "vertical_align": "top",
        "padding": "12px 12px 12px 12px",
        "elements": [
            {
                "tag": "column_set",
                "horizontal_spacing": "8px",
                "horizontal_align": "left",
                "columns": [
                    {
                        "tag": "column",
                        "width": "weighted",
                        "elements": [
                            {
                                "tag": "markdown",
                                "content": "%s",
                                "text_align": "left",
                                "text_size": "normal_v2"
                            }
                        ],
                        "vertical_spacing": "8px",
                        "horizontal_align": "left",
                        "vertical_align": "top",
                        "weight": 1
                    },
                    {
                        "tag": "column",
                        "width": "weighted",
                        "elements": [
                            {
                                "tag": "markdown",
                                "content": "%s",
                                "text_align": "left",
                                "text_size": "normal_v2"
                            }
                        ],
                        "vertical_spacing": "8px",
                        "horizontal_align": "left",
                        "vertical_align": "top",
                        "weight": 1
                    }
                ],
                "margin": "0px 0px 0px 0px"
            },




            {
                "tag": "column_set",
                "horizontal_spacing": "8px",
                "horizontal_align": "left",
                "columns": [
                    {
                        "tag": "column",
                        "width": "weighted",
                        "elements": [
                            {
                                "tag": "markdown",
                                "content": "%s",
                                "text_align": "left",
                                "text_size": "normal_v2"
                            }
                        ],
                        "vertical_spacing": "8px",
                        "horizontal_align": "left",
                        "vertical_align": "top",
                        "weight": 1
                    },
                    {
                        "tag": "column",
                        "width": "weighted",
                        "elements": [
                            {
                                "tag": "markdown",
                                "content": "%s",
                                "text_align": "left",
                                "text_size": "normal_v2"
                            }
                        ],
                        "vertical_spacing": "8px",
                        "horizontal_align": "left",
                        "vertical_align": "top",
                        "weight": 1
                    }
                ],
                "margin": "0px 0px 0px 0px"
            },
            {
                "tag": "column_set",
                "columns": [
                    {
                        "tag": "column",
                        "width": "weighted",
                        "elements": [
                            {
                                "tag": "markdown",
                                "content": "%s",
                                "text_align": "left",
                                "text_size": "normal_v2"
                            }
                        ],
                        "vertical_align": "top",
                        "weight": 1
                    },
                    {
                        "tag": "column",
                        "width": "weighted",
                        "elements": [
                            {
                                "tag": "markdown",
                                "content": "%s",
                                "text_align": "left",
                                "text_size": "normal_v2"
                            }
                        ],
                        "vertical_align": "top",
                        "weight": 1
                    }
                ]
            },
            {
                "tag": "column_set",
                "columns": [
                    {
                        "tag": "column",
                        "width": "weighted",
                        "elements": [
                            {
                                "tag": "markdown",
                                "content": "%s",
                                "text_align": "left",
                                "text_size": "normal_v2"
                            }
                        ],
                        "vertical_align": "top",
                        "weight": 1
                    },
                    {
                        "tag": "column",
                        "width": "weighted",
                        "elements": [
                            {
                                "tag": "markdown",
                                "content": "%s",
                                "text_align": "left",
                                "text_size": "normal_v2"
                            }
                        ],
                        "vertical_align": "top",
                        "weight": 1
                    }
                ]
            },
            {
                "tag": "column_set",
                "columns": [
                    {
                        "tag": "column",
                        "width": "weighted",
                        "elements": [
                            {
                                "tag": "markdown",
                                "content": "%s",
                                "text_align": "left",
                                "text_size": "normal_v2"
                            }
                        ],
                        "vertical_align": "top",
                        "weight": 1
                    },
                    {
                        "tag": "column",
                        "width": "weighted",
                        "elements": [
                            {
                                "tag": "markdown",
                                "content": "%s",
                                "text_align": "left",
                                "text_size": "normal_v2"
                            }
                        ],
                        "vertical_align": "top",
                        "weight": 1
                    }
                ]
            },
            {
                "tag": "hr",
                "margin": "0px 0px 0px 0px"
            },
            {
                "tag": "markdown",
                "content": "%s",
                "text_align": "left",
                "text_size": "normal_v2",
                "margin": "0px 0px 0px 0px"
            },
            {
                "tag": "hr",
                "margin": "0px 0px 0px 0px"
            },
            {
                "tag": "markdown",
                "content": "🔴 告警屏蔽按钮",
                "text_align": "left",
                "text_size": "normal_v2",
                "margin": "0px 0px 4px 0px"
            },
            {
                "tag": "column_set",
                "horizontal_spacing": "8px",
                "horizontal_align": "left",
                "columns": [
                    {
                        "tag": "column",
                        "width": "weighted",
                        "background_style": "bg-white",
                        "elements": [
                            {
                                "tag": "button",
                                "text": {
                                    "tag": "plain_text",
                                    "content": "认领告警"
                                },
                                "type": "primary",
                                "width": "default",
                                "size": "medium",
								"confirm": {
                                    "title": {
                                        "tag": "plain_text",
                                        "content": "确认认领吗？"
                                    }
                                },
								"behaviors": [
                                    {
                                        "type": "open_url",
                                        "default_url": "%s"
                                    }
                                ]
                            },
                            {
                                "tag": "button",
                                "text": {
                                    "tag": "plain_text",
                                    "content": "屏蔽 1 小时"
                                },
                                "type": "default",
                                "width": "default",
                                "size": "medium",
								"confirm": {
                                    "title": {
                                        "tag": "plain_text",
                                        "content": "确认屏蔽吗？"
                                    }
                                },
								"behaviors": [
                                    {
                                        "type": "open_url",
                                        "default_url": "%s"
                                    }
                                ]
                            },
                            {
                                "tag": "button",
                                "text": {
                                    "tag": "plain_text",
                                    "content": "屏蔽 1 天"
                                },
                                "type": "danger",
                                "width": "default",
                                "size": "medium",
								"confirm": {
                                    "title": {
                                        "tag": "plain_text",
                                        "content": "确认屏蔽吗？"
                                    }
                                },
								"behaviors": [
                                    {
                                        "type": "open_url",
                                        "default_url": "%s"
                                    }
                                ]
                            }











                        ],
                        "padding": "0px 0px 0px 0px",
                        "direction": "horizontal",
                        "horizontal_spacing": "8px",
                        "vertical_spacing": "8px",
                        "horizontal_align": "left",
                        "vertical_align": "top",
                        "margin": "0px 0px 0px 0px",
                        "weight": 3
                    }
                ],
                "margin": "0px 0px 4px 0px"
            },
			{
                "tag": "column_set",
                "horizontal_spacing": "8px",
                "horizontal_align": "left",
                "columns": [
                    {
                        "tag": "column",
                        "width": "weighted",
                        "background_style": "bg-white",
                        "elements": [
                            {
                                "tag": "button",
                                "text": {
                                    "tag": "plain_text",
                                    "content": "取消屏蔽"
                                },
                                "type": "primary",
                                "width": "default",
                                "size": "medium",
								"confirm": {
                                    "title": {
                                        "tag": "plain_text",
                                        "content": "确认取消屏蔽吗？"
                                    }
                                },
								"behaviors": [
                                    {
                                        "type": "open_url",
                                        "default_url": "%s"
                                    }
                                ]
                            },
                            {
                                "tag": "button",
                                "text": {
                                    "tag": "plain_text",
                                    "content": "屏蔽 6 小时"
                                },
                                "type": "default",
                                "width": "default",
                                "size": "medium",
								"confirm": {
                                    "title": {
                                        "tag": "plain_text",
                                        "content": "确认屏蔽吗？"
                                    }
                                },
								"behaviors": [
                                    {
                                        "type": "open_url",
                                        "default_url": "%s"
                                    }
                                ]
                            },
                            {
                                "tag": "button",
                                "text": {
                                    "tag": "plain_text",
                                    "content": "屏蔽 7 天"
                                },
                                "type": "danger",
                                "width": "default",
                                "size": "medium",
								"confirm": {
                                    "title": {
                                        "tag": "plain_text",
                                        "content": "确认屏蔽吗？"
                                    }
                                },
								"behaviors": [
                                    {
                                        "type": "open_url",
                                        "default_url": "%s"
                                    }
                                ]
                            },
                            {
                                "tag": "overflow",
                                "width": "default",
                                "options": [
                                    {
                                        "text": {
                                            "tag": "plain_text",
                                            "content": "屏蔽1小时"
                                        },
                                        "multi_url": {
                                            "url": "%s"
                                        }
                                    },
                                    {
                                        "text": {
                                            "tag": "plain_text",
                                            "content": "屏蔽6小时"
                                        },
                                        "multi_url": {
                                            "url": "%s"
                                        }
                                    },
                                    {
                                        "text": {
                                            "tag": "plain_text",
                                            "content": "屏蔽1天"
                                        },
                                        "multi_url": {
                                            "url": "%s"
                                        }
                                    },
                                    {
                                        "text": {
                                            "tag": "plain_text",
                                            "content": "屏蔽7天"
                                        },
                                        "multi_url": {
                                            "url": "%s"
                                        }
                                    }
                                ]
                            }



                        ],
                        "padding": "0px 0px 0px 0px",
                        "direction": "horizontal",
                        "horizontal_spacing": "8px",
                        "vertical_spacing": "8px",
                        "horizontal_align": "left",
                        "vertical_align": "top",
                        "margin": "0px 0px 0px 0px",
                        "weight": 3
                    }
                ],
                "margin": "0px 0px 4px 0px"
            },
            {
                "tag": "hr",
                "margin": "0px 0px 0px 0px"
            },
            {
                "tag": "markdown",
                "content": "**🙋 [我要反馈误报](https://www.qq.com) | 📝 [录入报警处理过程](https://www.qq.com)**",
                "text_align": "left",
                "text_size": "normal_v2",
                "margin": "0px 0px 0px 0px"
            }
        ]
    },
    "header": {
        "template": "%s",
        "padding": "12px 12px 12px 12px",
        "title": {
            "tag": "plain_text",
            "content": "%s"
        },
        "subtitle": {
            "tag": "plain_text",
            "content": ""
        }
    }
}`
	feiShuQunDataQun = `{
"msg_type": "interactive",
"card": %s
}`
)

func (ac *AlertCache) GenerateFeiShuCardMsgOneAlert(alert template.Alert, event *models.MonitorAlertEvent, rule *models.MonitorPromAlertRule, sendGroup *models.MonitorAlertManagerSendGroup) {
	//msgQun := fmt.Sprintf(feiShuQunDataQun, alert.Labels[common.MONITOR_ALERT_NAME_KEY]+alert.Fingerprint)
	//ac.SentFeiShuQun(msgQun)

	// 时间格式化 utc+8
	locName := ac.Sc.AlertTimezone
	if locName == "" {
		locName = "Asia/Shanghai"
	}
	loc, _ := time.LoadLocation(locName)

	// 告警标题
	alertHeader := fmt.Sprintf("[触发次数:%d]告警标题:%s 当前值 %s",
		event.EventTimes,
		alert.Labels[common.MONITOR_ALERT_NAME_KEY],
		alert.Annotations[common.MONITOR_ALERT_RULE_ANNO_VALUE],
	)
	// 告警级别
	severity := alert.Labels[common.MONITOR_ALERT_SEVERITY_KEY]
	streeNode := alert.Labels[common.MONITOR_ALERT_BIND_NODE_KEY]

	msgSeverity := fmt.Sprintf("**🚨告警级别:**\\n%s", severity)
	msgStatus := fmt.Sprintf("**🚥当前状态：**\\n<font color='%s'>**%s**</font>",
		common.MONITOR_ALERT_STATUS_COLOR_MAP[alert.Status],
		common.MONITOR_ALERT_STATUS_CH_MAP[alert.Status])
	msgStreeNode := fmt.Sprintf("**🌲绑定的服务树：**\\n<font color='green'>**%s**</font>", streeNode)
	msgTime := fmt.Sprintf("**🕒触发时间：**\\n%s", alert.StartsAt.In(loc).Format("2006-01-02 15:04:05"))
	var msgGrafana, msgExpr string
	if rule != nil {
		msgGrafana = fmt.Sprintf("**📈grafana：**\\n[链接](%s)", rule.GrafanaLink)
		msgExpr = fmt.Sprintf("<font color='green'>**🔀修改告警规则**</font>  [规则地址](%s)\\n<font color='red'>%s</font>",
			fmt.Sprintf("%s/%s?ruleId=%v", ac.Sc.FrontDomain, "monitor/rule/detail", rule.ID),
			rule.Expr)
	}

	// 私聊userIds列表
	siliaoUserIds := map[string]string{}

	alertHeaderColor := common.MONITOR_ALERT_SEVERITY_TITLE_COLOR_MAP[alert.Labels[common.MONITOR_ALERT_SEVERITY_KEY]] // 告警标题颜色

	// 获取值班组信息
	msgOnduty := "值班组和值班人（今日无人值班）"
	yuanshiRen := ""
	//user := "b75ag4g4"
	onDutyGroup := ac.GetOnDutyGroupById(sendGroup.OnDutyGroupId)
	onDutyGroupUrl := fmt.Sprintf("%s/%s?id=%v", ac.Sc.FrontDomain, "monitor/onduty/detail", sendGroup.ID)
	if onDutyGroup != nil {
		onDutyGroup.FillToDayOndutyUser()
		if onDutyGroup.ToDayOnDutyUser != nil {
			yuanshiRen = onDutyGroup.ToDayOnDutyUser.RealName
			msgOnduty = fmt.Sprintf("**👥值班组 [%s](%s)**\\n 值班人:%s user_id=%s<at id=%s></at>",
				onDutyGroup.Name,
				onDutyGroupUrl,
				onDutyGroup.ToDayOnDutyUser.RealName,
				onDutyGroup.ToDayOnDutyUser.FeiShuUserId,
				onDutyGroup.ToDayOnDutyUser.FeiShuUserId,
			)
			siliaoUserIds[onDutyGroup.ToDayOnDutyUser.FeiShuUserId] = ""
		}
	}

	// 判断告警升级
	msgUpgrade := "**⬆️升级状态：**\\n未升级"
	if alert.Status == common.MONITOR_ALERT_STATUS_FIRING && sendGroup.FirstUpgradeUsers != nil && len(sendGroup.FirstUpgradeUsers) > 0 {
		// 判断时间 当前时间 - 第一次触发时间 > upgrade时间
		if sendGroup.UpgradeMinutes == 0 {
			sendGroup.UpgradeMinutes = 30
		}
		if time.Now().Sub(alert.StartsAt) > time.Minute*time.Duration(sendGroup.UpgradeMinutes) {
			upgredeUserNames := ""
			upgredeUserAtIds := ""
			for _, user := range sendGroup.FirstUpgradeUsers {
				user := user
				siliaoUserIds[user.FeiShuUserId] = ""
				upgredeUserNames = fmt.Sprintf("%s %s", upgredeUserNames, user.RealName)
				upgredeUserAtIds = fmt.Sprintf("%s <at id=%s></at>", upgredeUserAtIds, user.FeiShuUserId)
			}
			msgUpgrade = fmt.Sprintf("**⬆️升级状态：**\\n`已升级`  [接收人变更]:[%s]->[%s]",
				yuanshiRen,
				upgredeUserNames,
			)
			// 群里at人变化
			msgOnduty = fmt.Sprintf("**👥值班组 [%s](%s)**\\n 告警升级人:%s",
				onDutyGroup.Name,
				onDutyGroupUrl,
				upgredeUserAtIds,
			)
		}

	}

	// 发送组
	msgSendGroupUrl := fmt.Sprintf("%s/%s?id=%v", ac.Sc.FrontDomain, "monitor/sendgroup/detail", sendGroup.ID)
	msgSendGroup := fmt.Sprintf("**✉️修改发送组：**\\n[%s](%s)", sendGroup.Name, msgSendGroupUrl)
	msgReLingUrl := fmt.Sprintf("%s/%s?fingerprint=%v", ac.Sc.BackendDomain, "reling", alert.Fingerprint)                    // 认领告警
	msgUnSilenceUrl := fmt.Sprintf("%s/%s?fingerprint=%v", ac.Sc.BackendDomain, "unsilence", alert.Fingerprint)              // 取消屏蔽
	msgSilenceOneHourUrl := fmt.Sprintf("%s/%s?fingerprint=%v&hour=1", ac.Sc.BackendDomain, "silence", alert.Fingerprint)    // 屏蔽 1 小时
	msgSilenceSexHourUrl := fmt.Sprintf("%s/%s?fingerprint=%v&hour=6", ac.Sc.BackendDomain, "silence", alert.Fingerprint)    // 屏蔽 6 小时
	msgSilenceOneDayUrl := fmt.Sprintf("%s/%s?fingerprint=%v&hour=24", ac.Sc.BackendDomain, "silence", alert.Fingerprint)    // 屏蔽 1 天
	msgSilenceSevenDayUrl := fmt.Sprintf("%s/%s?fingerprint=%v&hour=168", ac.Sc.BackendDomain, "silence", alert.Fingerprint) // 屏蔽 7 天

	// 基于alertname名称屏蔽
	msgSilenceOneHourByNameUrl := fmt.Sprintf("%s/%s?fingerprint=%v&hour=1&by_name=1", ac.Sc.BackendDomain, "silence", alert.Fingerprint)    // 屏蔽 1 小时
	msgSilenceSexHourByNameUrl := fmt.Sprintf("%s/%s?fingerprint=%v&hour=6&by_name=1", ac.Sc.BackendDomain, "silence", alert.Fingerprint)    // 屏蔽 6 小时
	msgSilenceOneDayByNameUrl := fmt.Sprintf("%s/%s?fingerprint=%v&hour=24&by_name=1", ac.Sc.BackendDomain, "silence", alert.Fingerprint)    // 屏蔽 1 天
	msgSilenceSevenDayByNameUrl := fmt.Sprintf("%s/%s?fingerprint=%v&hour=168&by_name=1", ac.Sc.BackendDomain, "silence", alert.Fingerprint) // 屏蔽 7 天

	// 告警标签
	labelsMap := alert.Labels
	delete(labelsMap, common.MONITOR_ALERT_NAME_KEY)
	delete(labelsMap, common.MONITOR_ALERT_SEVERITY_KEY)
	delete(labelsMap, common.MONITOR_ALERT_BIND_NODE_KEY)
	delete(labelsMap, common.MONITOR_ALERT_MATCH_KEY)
	delete(labelsMap, common.MONITOR_ALERT_RULE_KEY)
	anno := alert.Annotations
	delete(anno, common.MONITOR_ALERT_RULE_ANNO_VALUE)
	msgLabels := fmt.Sprintf("**标签信息:**\\n%s", common.GenKvStringByMap(labelsMap))
	msgAnnotations := fmt.Sprintf("**注解信息:**\\n%s", common.GenKvStringByMap(anno))

	msgSi := fmt.Sprintf(feiShuCardContent,
		msgLabels, msgAnnotations,
		msgSeverity, msgStatus, msgStreeNode, msgTime, msgUpgrade, msgOnduty, msgGrafana, msgSendGroup, msgExpr,
		msgReLingUrl, msgSilenceOneHourUrl, msgSilenceOneDayUrl, msgUnSilenceUrl, msgSilenceSexHourUrl, msgSilenceSevenDayUrl,
		msgSilenceOneHourByNameUrl, msgSilenceSexHourByNameUrl, msgSilenceOneDayByNameUrl, msgSilenceSevenDayByNameUrl,
		alertHeaderColor, alertHeader)
	//msgSilenceOneHourByNameUrl, msgSilenceSexHourByNameUrl, msgSilenceOneDayByNameUrl, msgSilenceSevenDayByNameUrl,

	ac.SentFeiShuPrivate(msgSi, siliaoUserIds) // 应用机器人

	//msgQun := fmt.Sprintf(feiShuQunDataQun, msgSi)
	//ac.SentFeiShuQun(msgQun) //发送群聊机器人
}

// SentFeiShuQun 飞书自定义机器人 群组
func (ac *AlertCache) SentFeiShuQun(msg string) {
	url := "https://open.feishu.cn/open-apis/bot/v2/hook/c5a63034-7a00-43e5-84c2-31b0661cc0ea"
	emptyMap := map[string]string{}
	respBytes, err := common.PostWithJsonString(ac.Sc.Logger, "SentFeiShuQun", 2, url, msg, emptyMap, emptyMap)
	fmt.Println(string(respBytes), err)

}

type FeiShuPrivateCardMsg struct {
	MsgType   string `json:"msg_type"`
	ReceiveId string `json:"receive_id"`
	Content   string `json:"content"`
}

// SentFeiShuPrivate 飞书app机器人 私聊
func (ac *AlertCache) SentFeiShuPrivate(cardContent string, siliaoUserId map[string]string) {
	for userId := range siliaoUserId {
		url := "https://open.feishu.cn/open-apis/im/v1/messages"
		params := map[string]string{"receive_id_type": "user_id"}
		tenantAccessToken := "t-g10479fUVOFXSWITIWQ3VXLHVG7CAOO7WT5A6FOO"
		headersMap := map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", tenantAccessToken),
			"Content-Type":  "application/json",
		}
		//feiShuUserId := "b75ag4g4"
		feiShuPrivateCardMsg := FeiShuPrivateCardMsg{
			MsgType:   "interactive",
			ReceiveId: userId,
			Content:   cardContent,
		}
		data, _ := json.Marshal(feiShuPrivateCardMsg)
		_, _ = common.PostWithJsonString(ac.Sc.Logger, "SentFeiShuPrivate", 2, url, string(data), params, headersMap)
		//if err != nil {
		//	ac.Sc.Logger.Error("发送飞书私聊失败", zap.Error(err), zap.String("userId", userId))
		//}
	}

}
