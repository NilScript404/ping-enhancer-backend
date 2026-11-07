package dto

type ConnectRequest struct {
	ConfigID int `json:"config_id"`
}

type ConfigItemResponse struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Location     string `json:"location"`
	RouteType    string `json:"route_type"`
	RouteAddress string `json:"route_address"`
}

type ConfigConnectResponse struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Location     string `json:"location"`
	RouteType    string `json:"route_type"`
	RouteAddress string `json:"route_address"`
	Value        string `json:"value"`
}
