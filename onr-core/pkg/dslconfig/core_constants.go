package dslconfig

const (
	exprChannelBaseURL   = "$channel.base_url"
	exprChannelKey       = "$channel.key"
	exprChannelLocation  = "$channel.location"
	exprCredentialProjID = "$credential.project_id"
	exprOAuthAccessToken = "$oauth.access_token"
	exprRequestModel     = "$request.model"
	exprRequestMapped    = "$request.model_mapped"
	exprTaskID           = "$task.id"
	exprTaskUpstreamID   = "$task.upstream_id"

	jsonOpSet           = "json_set"
	jsonOpReplace       = "json_replace"
	jsonOpSetIfAbsent   = "json_set_if_absent"
	jsonOpDel           = "json_del"
	jsonOpDelIfMissing  = "json_del_if_missing"
	jsonOpRename        = "json_rename"
	jsonOpWrapInputText = "json_wrap_input_text"
	jsonOpSetHeaderVals = "json_set_header_values"
	jsonOpFilterValues  = "json_filter_values"
	jsonOpDelWithCond   = "json_del_with_condition"
)
