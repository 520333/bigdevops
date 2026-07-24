package models

import (
	"bigdevops/src/common"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
)

type K8sInstance struct {
	Model
	Name   string `json:"name" gorm:"uniqueIndex:idx_app_instance_name;type:varchar(100);comment:实例英文名称"`
	UserID uint   `json:"userId"`

	ContainerCore
	Image    string `json:"image"`
	Replicas int    `json:"replicas"`

	WorkloadType  string `json:"workloadType" gorm:"type:varchar(50);default:'Deployment';comment:工作负载类型"`
	Env           string `json:"env" gorm:"type:varchar(50);default:'prod';comment:环境"`
	EnableSvc     bool   `json:"enableSvc" gorm:"default:false;comment:开启Service"`
	SvcType       string `json:"svcType" gorm:"type:varchar(50);default:'ClusterIP';comment:Service类型"`
	EnableIngress bool   `json:"enableIngress" gorm:"default:false;comment:开启Ingress"`
	IngressHost   string `json:"ingressHost" gorm:"type:varchar(255);comment:Ingress域名"`
	ConfigMapName string `json:"configMapName" gorm:"type:varchar(100);comment:关联ConfigMap"`
	SecretName    string `json:"secretName" gorm:"type:varchar(100);comment:关联Secret"`

	K8sAppId       uint   `json:"k8sAppId"`
	CreateUserName string `json:"createUserName" gorm:"-"`
	NodePath       string `json:"nodePath" gorm:"-"`

	Key           string  `json:"key" gorm:"-"` // 前端表格使用
	K8sAppObj     *K8sApp `json:"k8sAppObj" gorm:"-"`
	AppName       string  `json:"appName" gorm:"-"`
	ReadyReplicas int32   `json:"readyReplicas" gorm:"-"`
	ClusterStatus string  `json:"clusterStatus" gorm:"-"`
}

func (obj *K8sInstance) Create() error {
	return Db.Create(obj).Error
}

func (obj *K8sInstance) DeleteOne() error {
	return Db.Select(clause.Associations).Unscoped().Delete(obj).Error
}

func (obj *K8sInstance) CreateOne() error {
	return Db.Create(obj).Error
}

func (obj *K8sInstance) UpdateOne() error {
	return Db.Where("id = ?", obj.ID).Updates(obj).Error
}

func GetK8sInstanceById(id int) (*K8sInstance, error) {
	var dbInstance K8sInstance

	err := Db.Where("id = ? ", id).First(&dbInstance).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("instance不存在")
		}
		return nil, fmt.Errorf("数据库错误%v", err)
	}
	return &dbInstance, nil
}

func (obj *K8sInstance) FillFrontAllData() {
	dbUser, _ := GetUserById(int(obj.UserID))
	if dbUser != nil {
		obj.CreateUserName = fmt.Sprintf("%s(%s)", dbUser.Username, dbUser.RealName)
	}
	obj.Key = fmt.Sprintf("%d", obj.ID)
	dbApp, _ := GetK8sAppById(int(obj.K8sAppId))
	if dbApp != nil {
		_ = dbApp.FillFrontAllData()
		obj.K8sAppObj = dbApp
		obj.AppName = dbApp.Name
	}
}

func GetK8sInstanceAll() (obj []*K8sInstance, err error) {
	err = Db.Model(&K8sInstance{}).Find(&obj).Error
	return
}

func GetK8sInstanceByIdsWithLimitOffset(ids []int, limit, offset int) (obj []*K8sInstance, err error) {
	if len(ids) == 0 {
		return nil, nil
	}
	err = Db.Where("id IN ?", ids).Limit(limit).Offset(offset).Find(&obj).Error
	return
}

func DeleteK8sInstanceById(id int) error {
	return Db.Unscoped().Delete(&K8sInstance{}, id).Error
}

// GetK8sInstanceListByNameAndCreator 分页查询，支持按名称和创建人模糊查询
func GetK8sInstanceListByNameAndCreator(name, creator string, limit, offset int) (obj []*K8sInstance, err error) {
	query := Db.Model(&K8sInstance{})

	// 1. 按名称模糊查询
	if name != "" {
		query = query.Where("k8s_instances.name LIKE ?", "%"+name+"%")
	}

	// 2. 按创建人模糊查询 (关联 User 表)
	if creator != "" {
		query = query.Joins("left join users on users.id = k8s_instances.user_id").
			Where("users.username LIKE ? OR users.real_name LIKE ?", "%"+creator+"%", "%"+creator+"%")
	}

	err = query.Limit(limit).Offset(offset).Find(&obj).Error
	return
}

// GetK8sInstanceCountByNameAndCreator 对应统计总数
func GetK8sInstanceCountByNameAndCreator(name, creator string) (int64, error) {
	var count int64
	query := Db.Model(&K8sInstance{}).Joins("left join users on users.id = k8s_instances.user_id")

	if name != "" {
		query = query.Where("k8s_instances.name LIKE ?", "%"+name+"%")
	}
	if creator != "" {
		query = query.Where("users.username LIKE ? OR users.real_name LIKE ?", "%"+creator+"%", "%"+creator+"%")
	}

	err := query.Count(&count).Error
	return count, err
}

func (obj *K8sInstance) GetK8sDeployment() (*appsv1.Deployment, error) {
	if obj.K8sAppObj == nil {
		return nil, fmt.Errorf("k8sAppObj is nil")
	}
	dep := &appsv1.Deployment{}
	dep.Name = fmt.Sprintf("%s-%s", obj.K8sAppObj.Name, obj.Name)
	dep.Namespace = obj.K8sAppObj.Namespace

	replicas := int32(obj.Replicas)
	if replicas <= 0 {
		replicas = 1
	}
	dep.Spec.Replicas = &replicas

	var cPort []corev1.ContainerPort
	for _, svcPort := range obj.K8sAppObj.PortJsonFront {
		one := corev1.ContainerPort{
			Name:          svcPort.Name,
			ContainerPort: int32(svcPort.TargetPort.IntValue()),
			Protocol:      svcPort.Protocol,
		}
		cPort = append(cPort, one)
	}
	c := corev1.Container{
		Name:  obj.Name,
		Image: obj.Image,
		Ports: cPort,
	}

	envM := common.GenMapByKvString(obj.K8sAppObj.Envs)
	envThis := common.GenMapByKvString(obj.Envs)
	for k, v := range envThis {
		envM[k] = v
	}
	var envVars []corev1.EnvVar
	for k, v := range envM {
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	c.Env = envVars

	labelM := common.GenMapByKvString(obj.K8sAppObj.Labels)
	labelThis := common.GenMapByKvString(obj.Labels)
	for k, v := range labelThis {
		labelM[k] = v
	}
	if labelM == nil {
		labelM = make(map[string]string)
	}
	resName := obj.GetK8sResourceName()
	labelM["app"] = resName

	dep.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: labelM,
	}
	dep.Spec.Template.ObjectMeta = metav1.ObjectMeta{
		Labels: labelM,
	}

	if obj.K8sAppObj.Commands != "" {
		c.Command = parseCommandString(obj.K8sAppObj.Commands)
	}
	if obj.Commands != "" {
		c.Command = parseCommandString(obj.Commands)
	}

	if obj.K8sAppObj.Args != "" {
		c.Args = parseCommandString(obj.K8sAppObj.Args)
	}
	if obj.Args != "" {
		c.Args = parseCommandString(obj.Args)
	}

	c.Resources.Requests = make(corev1.ResourceList)
	c.Resources.Limits = make(corev1.ResourceList)

	if obj.K8sAppObj.CpuRequest != "" {
		if q, err := resource.ParseQuantity(obj.K8sAppObj.CpuRequest); err == nil {
			c.Resources.Requests[corev1.ResourceCPU] = q
		}
	}
	if obj.K8sAppObj.MemoryRequest != "" {
		if q, err := resource.ParseQuantity(obj.K8sAppObj.MemoryRequest); err == nil {
			c.Resources.Requests[corev1.ResourceMemory] = q
		}
	}
	if obj.K8sAppObj.CpuLimit != "" {
		if q, err := resource.ParseQuantity(obj.K8sAppObj.CpuLimit); err == nil {
			c.Resources.Limits[corev1.ResourceCPU] = q
		}
	}
	if obj.K8sAppObj.MemoryLimit != "" {
		if q, err := resource.ParseQuantity(obj.K8sAppObj.MemoryLimit); err == nil {
			c.Resources.Limits[corev1.ResourceMemory] = q
		}
	}

	if obj.CpuRequest != "" {
		if q, err := resource.ParseQuantity(obj.CpuRequest); err == nil {
			c.Resources.Requests[corev1.ResourceCPU] = q
		}
	}
	if obj.MemoryRequest != "" {
		if q, err := resource.ParseQuantity(obj.MemoryRequest); err == nil {
			c.Resources.Requests[corev1.ResourceMemory] = q
		}
	}
	if obj.CpuLimit != "" {
		if q, err := resource.ParseQuantity(obj.CpuLimit); err == nil {
			c.Resources.Limits[corev1.ResourceCPU] = q
		}
	}
	if obj.MemoryLimit != "" {
		if q, err := resource.ParseQuantity(obj.MemoryLimit); err == nil {
			c.Resources.Limits[corev1.ResourceMemory] = q
		}
	}

	dep.Spec.Template.Spec.Containers = []corev1.Container{c}

	return dep, nil
}

func (obj *K8sInstance) GetK8sStatefulSet() (*appsv1.StatefulSet, error) {
	dep, err := obj.GetK8sDeployment()
	if err != nil {
		return nil, err
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: dep.ObjectMeta,
		Spec: appsv1.StatefulSetSpec{
			Replicas:    dep.Spec.Replicas,
			Selector:    dep.Spec.Selector,
			Template:    dep.Spec.Template,
			ServiceName: obj.Name,
		},
	}
	return sts, nil
}

func (obj *K8sInstance) GetK8sDaemonSet() (*appsv1.DaemonSet, error) {
	dep, err := obj.GetK8sDeployment()
	if err != nil {
		return nil, err
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: dep.ObjectMeta,
		Spec: appsv1.DaemonSetSpec{
			Selector: dep.Spec.Selector,
			Template: dep.Spec.Template,
		},
	}
	return ds, nil
}

func (obj *K8sInstance) GetK8sStandalonePod() (*corev1.Pod, error) {
	dep, err := obj.GetK8sDeployment()
	if err != nil {
		return nil, err
	}
	pod := &corev1.Pod{
		ObjectMeta: dep.Spec.Template.ObjectMeta,
		Spec:       dep.Spec.Template.Spec,
	}
	return pod, nil
}

func (obj *K8sInstance) GetK8sResourceName() string {
	if obj.K8sAppObj != nil && obj.K8sAppObj.Name != "" {
		return fmt.Sprintf("%s-%s", obj.K8sAppObj.Name, obj.Name)
	}
	return obj.Name
}

func (obj *K8sInstance) GetK8sService() (*corev1.Service, error) {
	if obj.K8sAppObj == nil {
		return nil, fmt.Errorf("实例对应的K8sApp对象未找到")
	}
	svcType := corev1.ServiceType(obj.SvcType)
	if svcType == "" {
		svcType = corev1.ServiceTypeClusterIP
	}

	resName := obj.GetK8sResourceName()

	var ports []corev1.ServicePort
	if len(obj.K8sAppObj.PortJsonFront) > 0 {
		ports = obj.K8sAppObj.PortJsonFront
	} else {
		ports = []corev1.ServicePort{
			{
				Name:       "http",
				Port:       80,
				TargetPort: intstr.FromInt(80),
				Protocol:   corev1.ProtocolTCP,
			},
		}
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resName,
			Namespace: obj.K8sAppObj.Namespace,
			Labels: map[string]string{
				"app": resName,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: svcType,
			Selector: map[string]string{
				"app": resName,
			},
			Ports: ports,
		},
	}
	return svc, nil
}

func (obj *K8sInstance) GetK8sIngress() (*networkingv1.Ingress, error) {
	if obj.K8sAppObj == nil || obj.IngressHost == "" {
		return nil, fmt.Errorf("实例对应的K8sApp或IngressHost为空")
	}
	pathType := networkingv1.PathTypePrefix
	resName := obj.GetK8sResourceName()

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resName,
			Namespace: obj.K8sAppObj.Namespace,
			Labels: map[string]string{
				"app": resName,
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: obj.IngressHost,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: resName,
											Port: networkingv1.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return ing, nil
}

// parseCommandString 智能解析命令行字符串，正确按空格/双引号/单引号拆分为字符串切片
func parseCommandString(cmdStr string) []string {
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return nil
	}
	var args []string
	var current strings.Builder
	inDoubleQuotes := false
	inSingleQuotes := false
	escaped := false

	for i := 0; i < len(cmdStr); i++ {
		r := cmdStr[i]

		if escaped {
			current.WriteByte(r)
			escaped = false
			continue
		}

		if r == '\\' && !inSingleQuotes {
			escaped = true
			continue
		}

		if r == '"' && !inSingleQuotes {
			inDoubleQuotes = !inDoubleQuotes
			continue
		}

		if r == '\'' && !inDoubleQuotes {
			inSingleQuotes = !inSingleQuotes
			continue
		}

		if (r == ' ' || r == '\t' || r == '\n') && !inDoubleQuotes && !inSingleQuotes {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(r)
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}
