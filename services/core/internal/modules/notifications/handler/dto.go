package handler

// RegisterDeviceRequest is request body for POST /notifications/devices.
type RegisterDeviceRequest struct {
	APNSToken   string  `json:"apns_token" binding:"required"`
	BundleID    string  `json:"bundle_id" binding:"required"`
	Environment string  `json:"environment" binding:"required,oneof=sandbox production"`
	AppVersion  *string `json:"app_version,omitempty"`
}

// RegisterLiveActivityRequest is request body for POST /notifications/orders/:orderId/live-activity.
type RegisterLiveActivityRequest struct {
	PushToken   string `json:"push_token" binding:"required"`
	BundleID    string `json:"bundle_id" binding:"required"`
	Environment string `json:"environment" binding:"required,oneof=sandbox production"`
}
