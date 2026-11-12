# BigDevOps 数据库初始化脚本说明

本目录包含 BigDevOps 平台的标准数据库初始化脚本，用于替代原有的代码 Mock 注入建库方式。

## 1. 文件说明

- `init.sql` / `bigdevops.sql`：
  - **DDL 结构**：全量 49 张业务与系统数据表的完整建表语句（字符集为 `utf8mb4`，排序规则为 `utf8mb4_unicode_ci`）。
  - **纯净基础数据**：
    - **系统用户**：超级管理员 `admin`（默认密码 `123456`）、系统自动化机器人账号 `auto_order_robot`、运维账号 `test`（已剔除所有测试用的 mock0~mock4 等假用户）。
    - **系统角色**：超级管理员（`super`）、运维人员（`ops`）、自动机器人（`bot_super`）、K8s集群管理员（`k8s_admin`）、普通用户（`user`）。
    - **菜单体系**：包含工作台、CMDB资产管理、工单服务、任务执行、监控中心、K8s集群运维、CI/CD流水线、系统管理等全量 61 项前台路由菜单。
    - **接口清单**：全量 331 项后端 REST API 权限路径定义。
    - **权限与规则**：Casbin 权限策略定义以及角色与菜单/API的完整权限映射关系。
    - **系统设置与预设**：基础系统配置、标准审批工单流模板、CI/CD流水线常用 Stage 模版等。

---

## 2. 使用方式

### 方式一：后端自动初始化（开箱即用）
当后端服务连接到一个全新的数据库（或尚未初始化的数据库）启动时，系统会自动检测 `system_users` 是否存在 `admin` 账号：
- 若**未初始化**：自动读取 `sql/init.sql` 执行建表并填充核心基础数据。
- 若**已初始化**：自动跳过 SQL 执行，不影响已有业务数据。

### 方式二：手动导入初始化（推荐用于正式生产环境）

在 MySQL 服务端或客户端直接执行以下命令：

```bash
# 1. 登录 MySQL 并创建数据库 (如未创建)
CREATE DATABASE IF NOT EXISTS bigdevops DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

# 2. 导入初始化 SQL 文件
mysql -h 127.0.0.1 -P 3306 -u root -p bigdevops < sql/init.sql
```

导入成功后，启动后端服务即可正常登录平台：
- **管理员账号**：`admin`
- **默认初始密码**：`123456`
