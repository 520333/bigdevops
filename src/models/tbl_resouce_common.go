package models

type ResourceCommon struct {
	Hash string `json:"hash" gorm:"uniqueIndex;type:varchar(128);comment:增量更新唯一标识"`
	//InstanceId string `json:"InstanceId,omitempty" gorm:"uniqueIndex;type:varchar(100);comment:实例id"`

	Vendor string `json:"Vendor" gorm:"type:varchar(200);comment:云厂商 阿里 华为 aws"`
	//VpcId  string `json:"VpcId,omitempty" gorm:"comment:专有网络VPC ID"`
	ZoneId string `json:"ZoneId,omitempty" gorm:"comment:实例可用区"`

	AccountName string      `json:"account_name"`
	Tags        StringArray `json:"Tags" gorm:"comment:标签集合[k1=v1,k2=v2]"`
	Env         string      `json:"Env" gorm:"comment:环境标识：dev开发 |test测试 |stage预发 |press压测 | prod生产"`
}
