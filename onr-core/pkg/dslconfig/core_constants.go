package dslconfig

const (
	exprChannelBaseURL   = "$channel.base_url"
	exprChannelKey       = "$channel.key"
	exprOAuthAccessToken = "$oauth.access_token"
	exprRequestModel     = "$request.model"
	exprRequestMapped    = "$request.model_mapped"

	jsonOpSet           = "json_set"
	jsonOpSetIfAbsent   = "json_set_if_absent"
	jsonOpDel           = "json_del"
	jsonOpRename        = "json_rename"
	jsonOpWrapInputText = "json_wrap_input_text"
	jsonOpSetHeaderVals = "json_set_header_values"
	jsonOpFilterValues  = "json_filter_values"
	jsonOpDelWithCond   = "json_del_with_condition"
)
