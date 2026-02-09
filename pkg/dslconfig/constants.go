package dslconfig

const (
	exprChannelBaseURL   = "$channel.base_url"
	exprChannelKey       = "$channel.key"
	exprOAuthAccessToken = "$oauth.access_token"
	exprRequestModel     = "$request.model"
	exprRequestMapped    = "$request.model_mapped"

	jsonOpSet         = "json_set"
	jsonOpSetIfAbsent = "json_set_if_absent"
	jsonOpDel         = "json_del"
	jsonOpRename      = "json_rename"
)
