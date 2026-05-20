package models

func CasBinCheckPermission(roleName, path, method string) (bool, error) {

	return CasbinEnforcer.Enforce(roleName, path, method)
}
