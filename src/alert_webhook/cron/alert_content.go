package cron

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bigdevops/src/common"
	"bigdevops/src/models"

	"github.com/prometheus/alertmanager/template"
	"go.uber.org/zap"
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

func (ac *AlertCache) GenerateFeiShuCardMsgOneAlert(alert template.Alert, event *models.MonitorAlertManagerEvent, rule *models.MonitorPromAlertRule, sendGroup *models.MonitorAlertManagerSendGroup) {
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
	if event.Status == common.MONITOR_ALERT_STATUS_RENLING {
		msgUpgrade = "**⬆️升级状态：**\\n<font color='green'>已认领，终止升级</font>"
	} else if event.Status == common.MONITOR_ALERT_STATUS_SILIENCED {
		msgUpgrade = "**⬆️升级状态：**\\n<font color='grey'>已屏蔽，终止升级</font>"
	} else if alert.Status == common.MONITOR_ALERT_STATUS_FIRING && sendGroup.FirstUpgradeUsers != nil && len(sendGroup.FirstUpgradeUsers) > 0 {
		// 只有真正还在 firing，且没被认领、没被屏蔽的告警，才进入超时升级判定
		if sendGroup.UpgradeMinutes == 0 {
			sendGroup.UpgradeMinutes = 30
		}

		effectiveStartTime := alert.StartsAt
		if event.UnsilencedAt != nil && event.UnsilencedAt.After(alert.StartsAt) {
			effectiveStartTime = *event.UnsilencedAt
		}
		// 判断超时
		if time.Now().Sub(effectiveStartTime) > time.Minute*time.Duration(sendGroup.UpgradeMinutes) {
			upgredeUserNames := ""
			upgredeUserAtIds := ""
			for _, user := range sendGroup.FirstUpgradeUsers {
				user := user
				siliaoUserIds[user.FeiShuUserId] = ""
				upgredeUserNames = fmt.Sprintf("%s %s", upgredeUserNames, user.RealName)
				upgredeUserAtIds = fmt.Sprintf("%s <at id=%s></at>", upgredeUserAtIds, user.FeiShuUserId)
			}
			msgUpgrade = fmt.Sprintf("**⬆️升级状态：**`已升级`\\n  [接收人变更]:[%s]->[%s]",
				yuanshiRen,
				upgredeUserNames,
			)
			// 群里at人变化
			msgOnduty = fmt.Sprintf("**👥值班组 [%s](%s)**\\n 告警升级人:%s",
				onDutyGroup.Name,
				onDutyGroupUrl,
				upgredeUserAtIds,
			)
			event.Status = common.MONITOR_ALERT_STATUS_UPGRADED
			_ = event.UpdateOne()
		}
	}
	// 判断认领：
	if event.ReLingUser != nil {
		msgOnduty = fmt.Sprintf("**👥值班组 [%s](%s)**\\n 认领人:%s user_id=%s<at id=%s></at>",
			onDutyGroup.Name,
			onDutyGroupUrl,
			event.ReLingUser.RealName,
			event.ReLingUser.FeiShuUserId,
			event.ReLingUser.FeiShuUserId,
		)
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

	// 告警标签（深拷贝，防止 delete 污染原始 alert.Labels）
	labelsMap := make(map[string]string, len(alert.Labels))
	for k, v := range alert.Labels {
		labelsMap[k] = v
	}
	delete(labelsMap, common.MONITOR_ALERT_NAME_KEY)
	delete(labelsMap, common.MONITOR_ALERT_SEVERITY_KEY)
	delete(labelsMap, common.MONITOR_ALERT_BIND_NODE_KEY)
	delete(labelsMap, common.MONITOR_ALERT_MATCH_KEY)
	delete(labelsMap, common.MONITOR_ALERT_RULE_KEY)

	anno := make(map[string]string, len(alert.Annotations))
	for k, v := range alert.Annotations {
		anno[k] = v
	}
	delete(anno, common.MONITOR_ALERT_RULE_ANNO_VALUE)
	msgLabels := fmt.Sprintf("**标签信息:**\\n%s", common.GenKvStringByMap(labelsMap))
	msgAnnotations := fmt.Sprintf("**注解信息:**\\n%s", common.GenKvStringByMap(anno))

	msgSi := fmt.Sprintf(feiShuCardContent,
		msgLabels, msgAnnotations,
		msgSeverity, msgStatus, msgStreeNode, msgTime, msgUpgrade, msgOnduty, msgGrafana, msgSendGroup, msgExpr,
		msgReLingUrl, msgSilenceOneHourUrl, msgSilenceOneDayUrl, msgUnSilenceUrl, msgSilenceSexHourUrl, msgSilenceSevenDayUrl,
		msgSilenceOneHourByNameUrl, msgSilenceSexHourByNameUrl, msgSilenceOneDayByNameUrl, msgSilenceSevenDayByNameUrl,
		alertHeaderColor, alertHeader)

	ac.SentFeiShuPrivate(msgSi, siliaoUserIds) // 应用机器人

	msgQun := fmt.Sprintf(feiShuQunDataQun, msgSi)
	ac.SentFeiShuQun(msgQun) //发送群聊机器人
}

// SentFeiShuQun 飞书自定义机器人 群组
func (ac *AlertCache) SentFeiShuQun(msg string) {
	url := ac.Sc.ImC.FeiShu.Webhook
	emptyMap := map[string]string{}
	respBytes, err := common.PostWithJsonString(ac.Sc.Logger, "SentFeiShuQun", ac.Sc.ImC.FeiShu.RequestTimeoutSeconds, url, msg, emptyMap, emptyMap)
	if err != nil {
		ac.Sc.Logger.Error("发送飞书群聊消息失败", zap.Error(err), zap.Any("结果", string(respBytes)))
	}
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
		tenantAccessToken := ac.GetPrivateChatToken()
		headersMap := map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", tenantAccessToken),
			"Content-Type":  "application/json",
		}
		feiShuPrivateCardMsg := FeiShuPrivateCardMsg{
			MsgType:   "interactive",
			ReceiveId: userId,
			Content:   cardContent,
		}
		data, _ := json.Marshal(feiShuPrivateCardMsg)
		respBytes, err := common.PostWithJsonString(ac.Sc.Logger, "SentFeiShuPrivate", ac.Sc.ImC.FeiShu.RequestTimeoutSeconds, url, string(data), params, headersMap)
		if err != nil {
			ac.Sc.Logger.Error("发送飞书私聊消息失败", zap.Error(err), zap.Any("结果", string(respBytes)), zap.Any("userId", userId))
		}
	}
}

// GenerateDingTalkMarkdownMsgOneAlert 生成钉钉告警并发送
func (ac *AlertCache) GenerateDingTalkMarkdownMsgOneAlert(alert template.Alert, event *models.MonitorAlertManagerEvent, rule *models.MonitorPromAlertRule, sendGroup *models.MonitorAlertManagerSendGroup) {
	if ac.Sc.ImC == nil || ac.Sc.ImC.DingDing == nil || !ac.Sc.ImC.DingDing.Enabled {
		return
	}
	webhook := ac.Sc.ImC.DingDing.Webhook
	secret := ac.Sc.ImC.DingDing.Secret
	if webhook == "" {
		return
	}

	// 时区设置
	locName := ac.Sc.AlertTimezone
	if locName == "" {
		locName = "Asia/Shanghai"
	}
	loc, _ := time.LoadLocation(locName)

	status := alert.Status
	startLocal := alert.StartsAt.In(loc).Format("2006-01-02 15:04:05")
	endLocal := ""
	if !alert.EndsAt.IsZero() && alert.EndsAt.Year() > 1970 {
		endLocal = alert.EndsAt.In(loc).Format("2006-01-02 15:04:05")
	}

	// 获取原始 description
	description := ""
	if desc, ok := alert.Annotations["description"]; ok {
		description = desc
	} else if desc, ok := alert.Annotations["summary"]; ok {
		description = desc
	}

	var title string
	switch status {
	case "firing":
		title = "异常消息"
	case "resolved":
		title = "恢复消息"
		description = fmt.Sprintf("实例 **%s** 的 **%s** 告警已解除，相关指标已恢复至正常范围内。", alert.Labels["instance"], alert.Labels["alertname"])
	default:
		title = "未知状态"
	}

	project := "sg"
	if sendGroup != nil && sendGroup.NameZh != "" {
		project = sendGroup.NameZh
	}

	alertName := alert.Labels["alertname"]
	if alertName == "" && event != nil {
		alertName = event.AlertName
	}
	if alertName == "" && rule != nil {
		alertName = rule.Name
	}

	severity := alert.Labels["severity"]
	if severity == "" && rule != nil {
		severity = rule.Severity
	}

	job := alert.Labels["job"]
	if job == "" && rule != nil {
		job = rule.Name
	}

	summary := alert.Annotations["summary"]
	if summary == "" {
		summary = alertName
	}

	// 格式与生产现有 webhook-dingtalk 完全一致
	messageText := fmt.Sprintf(
		"##### <font color=#A9A9A9>告警指标:</font>%v\n"+
			"##### <font color=#A9A9A9>告警类型:</font>%v\n"+
			"##### <font color=#A9A9A9>告警级别:</font>%v\n"+
			"##### <font color=#A9A9A9>所属项目:</font>%s\n"+
			"##### <font color=#A9A9A9>主题:</font>%v\n"+
			"##### <font color=#A9A9A9>告警详情:</font>\n"+
			">##### <font color=#FF0000>**%v**</font>\n"+
			"##### <font color=#A9A9A9>告警时间:</font><font color=#FFD700>**%s**</font>\n",
		job, alertName, severity,
		project, summary, description,
		startLocal,
	)

	// 如果关联了服务树
	if streeNode, ok := alert.Labels[common.MONITOR_ALERT_BIND_NODE_KEY]; ok && streeNode != "" {
		messageText += fmt.Sprintf("##### <font color=#A9A9A9>服务树节点:</font><font color=#00CD00>%s</font>\n", streeNode)
	}

	// 仅 resolved 告警才显示恢复时间
	if status == "resolved" && endLocal != "" {
		messageText += fmt.Sprintf("##### <font color=#A9A9A9>恢复时间:</font><font color=#00CD00>**%s**</font>\n", endLocal)
	}

	ac.SentDingTalkMarkdown(webhook, secret, title, messageText)
}

// SentDingTalkMarkdown 发送钉钉 Markdown 消息 (支持 HMAC-SHA256 签名)
func (ac *AlertCache) SentDingTalkMarkdown(webhook, secret, title, text string) {
	webhookURL := webhook
	if secret != "" {
		timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
		stringToSign := fmt.Sprintf("%s\n%s", timestamp, secret)
		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(stringToSign))
		sign := base64.StdEncoding.EncodeToString(h.Sum(nil))
		sign = url.QueryEscape(sign)
		sep := "&"
		if !strings.Contains(webhook, "?") {
			sep = "?"
		}
		webhookURL = fmt.Sprintf("%s%stimestamp=%s&sign=%s", webhook, sep, timestamp, sign)
	}

	payloadMap := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  text,
		},
	}
	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		ac.Sc.Logger.Error("钉钉消息 JSON 序列化失败", zap.Error(err))
		return
	}

	timeout := 5
	if ac.Sc.ImC != nil && ac.Sc.ImC.DingDing != nil && ac.Sc.ImC.DingDing.RequestTimeoutSeconds > 0 {
		timeout = ac.Sc.ImC.DingDing.RequestTimeoutSeconds
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		ac.Sc.Logger.Error("创建钉钉 HTTP 请求失败", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")

	resp, err := client.Do(req)
	if err != nil {
		ac.Sc.Logger.Error("发送钉钉告警失败", zap.Error(err), zap.String("url", webhookURL))
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	ac.Sc.Logger.Info("钉钉告警响应", zap.Int("statusCode", resp.StatusCode), zap.String("response", string(respBody)))
}
