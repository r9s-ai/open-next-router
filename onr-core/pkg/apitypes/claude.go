package apitypes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type ClaudeMetadata struct {
	UserId string `json:"user_id,omitempty"`
}

func (c *ClaudeMetadata) FromMap(m map[string]any) error {
	var err error
	c.UserId, err = stringValue(m, "user_id")
	return err
}

func (c *ClaudeMetadata) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "user_id", c.UserId)
	return out, nil
}

type ClaudeBaseSource struct {
	Type string `json:"type"`
}

func (c *ClaudeBaseSource) GetType() string { return c.Type }

func (c *ClaudeBaseSource) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	return err
}

func (c *ClaudeBaseSource) ToMap() (map[string]any, error) {
	return map[string]any{"type": c.Type}, nil
}

type ClaudeBase64Source struct {
	ClaudeBaseSource
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

func (c *ClaudeBase64Source) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseSource.FromMap(m); err != nil {
		return err
	}
	var err error
	c.MediaType, err = stringValue(m, "media_type")
	if err != nil {
		return err
	}
	c.Data, err = stringValue(m, "data")
	return err
}

func (c *ClaudeBase64Source) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseSource.ToMap()
	out["media_type"] = c.MediaType
	out["data"] = c.Data
	return out, nil
}

type ClaudeTextSource struct {
	ClaudeBaseSource
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

func (c *ClaudeTextSource) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseSource.FromMap(m); err != nil {
		return err
	}
	var err error
	c.MediaType, err = stringValue(m, "media_type")
	if err != nil {
		return err
	}
	c.Data, err = stringValue(m, "data")
	return err
}

func (c *ClaudeTextSource) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseSource.ToMap()
	out["media_type"] = c.MediaType
	out["data"] = c.Data
	return out, nil
}

type ClaudeURLSource struct {
	ClaudeBaseSource
	URL string `json:"url"`
}

func (c *ClaudeURLSource) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseSource.FromMap(m); err != nil {
		return err
	}
	var err error
	c.URL, err = stringValue(m, "url")
	return err
}

func (c *ClaudeURLSource) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseSource.ToMap()
	out["url"] = c.URL
	return out, nil
}

type ClaudeContentSource struct {
	ClaudeBaseSource
	Content string `json:"content"`
}

func (c *ClaudeContentSource) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseSource.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Content, err = stringValue(m, "content")
	return err
}

func (c *ClaudeContentSource) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseSource.ToMap()
	out["content"] = c.Content
	return out, nil
}

type ClaudeUnknownSource struct {
	ClaudeBaseSource                 // carries the raw type string for any unrecognized source type
	Raw              json.RawMessage `json:"-"`
}

func (s *ClaudeUnknownSource) MarshalJSON() ([]byte, error) {
	if len(s.Raw) > 0 {
		return s.Raw, nil
	}
	return json.Marshal(s.ClaudeBaseSource)
}

type ClaudeSource interface {
	GetType() string
}

type CacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

func (c *CacheControl) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	c.TTL, err = stringValue(m, "ttl")
	return err
}

func (c *CacheControl) ToMap() (map[string]any, error) {
	out := map[string]any{"type": c.Type}
	setMapString(out, "ttl", c.TTL)
	return out, nil
}

type ClaudeBaseCitation struct {
	Type string `json:"type"`
}

func (c *ClaudeBaseCitation) GetType() string { return c.Type }

func (c *ClaudeBaseCitation) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	return err
}

func (c *ClaudeBaseCitation) ToMap() (map[string]any, error) {
	return map[string]any{"type": c.Type}, nil
}

type ClaudeCitationCharLocation struct {
	ClaudeBaseCitation
	CitedText      string `json:"cited_text"`
	DocumentIndex  int    `json:"document_index"`
	DocumentTitle  string `json:"document_title"`
	EndCharIndex   int    `json:"end_char_index"`
	StartCharIndex int    `json:"start_char_index"`
}

func (c *ClaudeCitationCharLocation) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseCitation.FromMap(m); err != nil {
		return err
	}
	var err error
	c.CitedText, err = stringValue(m, "cited_text")
	if err != nil {
		return err
	}
	c.DocumentIndex, err = intValue(m, "document_index")
	if err != nil {
		return err
	}
	c.DocumentTitle, err = stringValue(m, "document_title")
	if err != nil {
		return err
	}
	c.EndCharIndex, err = intValue(m, "end_char_index")
	if err != nil {
		return err
	}
	c.StartCharIndex, err = intValue(m, "start_char_index")
	return err
}

func (c *ClaudeCitationCharLocation) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseCitation.ToMap()
	out["cited_text"] = c.CitedText
	out["document_index"] = c.DocumentIndex
	out["document_title"] = c.DocumentTitle
	out["end_char_index"] = c.EndCharIndex
	out["start_char_index"] = c.StartCharIndex
	return out, nil
}

type ClaudeCitationPageLocation struct {
	ClaudeBaseCitation
	CitedText      string `json:"cited_text"`
	DocumentIndex  int    `json:"document_index"`
	DocumentTitle  string `json:"document_title"`
	EndPageIndex   int    `json:"end_page_index"`
	StartPageIndex int    `json:"start_page_index"`
}

func (c *ClaudeCitationPageLocation) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseCitation.FromMap(m); err != nil {
		return err
	}
	var err error
	c.CitedText, err = stringValue(m, "cited_text")
	if err != nil {
		return err
	}
	c.DocumentIndex, err = intValue(m, "document_index")
	if err != nil {
		return err
	}
	c.DocumentTitle, err = stringValue(m, "document_title")
	if err != nil {
		return err
	}
	c.EndPageIndex, err = intValue(m, "end_page_index")
	if err != nil {
		return err
	}
	c.StartPageIndex, err = intValue(m, "start_page_index")
	return err
}

func (c *ClaudeCitationPageLocation) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseCitation.ToMap()
	out["cited_text"] = c.CitedText
	out["document_index"] = c.DocumentIndex
	out["document_title"] = c.DocumentTitle
	out["end_page_index"] = c.EndPageIndex
	out["start_page_index"] = c.StartPageIndex
	return out, nil
}

type ClaudeCitationContentBlockLocation struct {
	ClaudeBaseCitation
	CitedText       string `json:"cited_text"`
	DocumentIndex   int    `json:"document_index"`
	DocumentTitle   string `json:"document_title"`
	EndBlockIndex   int    `json:"end_block_index"`
	StartBlockIndex int    `json:"start_block_index"`
}

func (c *ClaudeCitationContentBlockLocation) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseCitation.FromMap(m); err != nil {
		return err
	}
	var err error
	c.CitedText, err = stringValue(m, "cited_text")
	if err != nil {
		return err
	}
	c.DocumentIndex, err = intValue(m, "document_index")
	if err != nil {
		return err
	}
	c.DocumentTitle, err = stringValue(m, "document_title")
	if err != nil {
		return err
	}
	c.EndBlockIndex, err = intValue(m, "end_block_index")
	if err != nil {
		return err
	}
	c.StartBlockIndex, err = intValue(m, "start_block_index")
	return err
}
func (c *ClaudeCitationContentBlockLocation) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseCitation.ToMap()
	out["cited_text"] = c.CitedText
	out["document_index"] = c.DocumentIndex
	out["document_title"] = c.DocumentTitle
	out["end_block_index"] = c.EndBlockIndex
	out["start_block_index"] = c.StartBlockIndex
	return out, nil
}

type ClaudeCitationWebSearchResultLocation struct {
	ClaudeBaseCitation
	CitedText      string `json:"cited_text"`
	EncryptedIndex string `json:"encrypted_index"`
	Title          string `json:"title"`
	URL            string `json:"url"`
}

func (c *ClaudeCitationWebSearchResultLocation) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseCitation.FromMap(m); err != nil {
		return err
	}
	var err error
	c.CitedText, err = stringValue(m, "cited_text")
	if err != nil {
		return err
	}
	c.EncryptedIndex, err = stringValue(m, "encrypted_index")
	if err != nil {
		return err
	}
	c.Title, err = stringValue(m, "title")
	if err != nil {
		return err
	}
	c.URL, err = stringValue(m, "url")
	return err
}
func (c *ClaudeCitationWebSearchResultLocation) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseCitation.ToMap()
	out["cited_text"] = c.CitedText
	out["encrypted_index"] = c.EncryptedIndex
	out["title"] = c.Title
	out["url"] = c.URL
	return out, nil
}

type ClaudeCitationSearchResultLocation struct {
	ClaudeBaseCitation
	CitedText         string `json:"cited_text"`
	EndBlockIndex     int    `json:"end_block_index"`
	StartBlockIndex   int    `json:"start_block_index"`
	Title             string `json:"title"`
	Source            string `json:"source"`
	SearchResultIndex int    `json:"search_result_index"`
}

func (c *ClaudeCitationSearchResultLocation) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseCitation.FromMap(m); err != nil {
		return err
	}
	var err error
	c.CitedText, err = stringValue(m, "cited_text")
	if err != nil {
		return err
	}
	c.EndBlockIndex, err = intValue(m, "end_block_index")
	if err != nil {
		return err
	}
	c.StartBlockIndex, err = intValue(m, "start_block_index")
	if err != nil {
		return err
	}
	c.Title, err = stringValue(m, "title")
	if err != nil {
		return err
	}
	c.Source, err = stringValue(m, "source")
	if err != nil {
		return err
	}
	c.SearchResultIndex, err = intValue(m, "search_result_index")
	return err
}
func (c *ClaudeCitationSearchResultLocation) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseCitation.ToMap()
	out["cited_text"] = c.CitedText
	out["end_block_index"] = c.EndBlockIndex
	out["start_block_index"] = c.StartBlockIndex
	out["title"] = c.Title
	out["source"] = c.Source
	out["search_result_index"] = c.SearchResultIndex
	return out, nil
}

type ClaudeUnknownCitation struct {
	ClaudeBaseCitation                 // carries the raw type string for any unrecognized citation type
	Raw                json.RawMessage `json:"-"`
}

func (c *ClaudeUnknownCitation) MarshalJSON() ([]byte, error) {
	if len(c.Raw) > 0 {
		return c.Raw, nil
	}
	return json.Marshal(c.ClaudeBaseCitation)
}

type ClaudeCitation interface {
	GetType() string
}

type ClaudeBaseContent struct {
	Type string `json:"type"`
}

func (c *ClaudeBaseContent) GetType() string { return c.Type }

func (c *ClaudeBaseContent) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	return err
}

func (c *ClaudeBaseContent) ToMap() (map[string]any, error) {
	return map[string]any{"type": c.Type}, nil
}

type ClaudeTextContent struct {
	ClaudeBaseContent
	Text         string           `json:"text"`
	CacheControl *CacheControl    `json:"cache_control,omitempty"`
	Citations    []ClaudeCitation `json:"citations,omitempty"`
}

func (c *ClaudeTextContent) UnmarshalJSON(b []byte) error {
	type alias ClaudeTextContent
	aux := struct {
		*alias
		Citations []json.RawMessage `json:"citations,omitempty"`
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	if len(aux.Citations) == 0 {
		c.Citations = nil
		return nil
	}
	items := make([]ClaudeCitation, 0, len(aux.Citations))
	for _, raw := range aux.Citations {
		item, err := decodeClaudeCitation(raw)
		if err != nil {
			return err
		}
		items = append(items, item)
	}
	c.Citations = items
	return nil
}

func (c *ClaudeTextContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Text, err = stringValue(m, "text")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	if err != nil {
		return err
	}
	c.Citations, err = decodeClaudeCitationListFromMapField(m, "citations")
	return err
}

func (c *ClaudeTextContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["text"] = c.Text
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	if len(c.Citations) > 0 {
		citations, err := claudeCitationListToMaps(c.Citations)
		if err != nil {
			return nil, err
		}
		out["citations"] = citations
	}
	return out, nil
}

type ClaudeImageContent struct {
	ClaudeBaseContent
	Source       ClaudeSource  `json:"source"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

func (c *ClaudeImageContent) UnmarshalJSON(b []byte) error {
	type alias ClaudeImageContent
	aux := struct {
		*alias
		Source json.RawMessage `json:"source"`
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	src, err := decodeClaudeSource(aux.Source)
	if err != nil {
		return err
	}
	c.Source = src
	return nil
}

func (c *ClaudeImageContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	src, err := decodeClaudeSourceMapField(m, "source")
	if err != nil {
		return err
	}
	c.Source = src
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	return err
}

func (c *ClaudeImageContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	source, err := claudeSourceToMap(c.Source)
	if err != nil {
		return nil, err
	}
	out["source"] = source
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	return out, nil
}

type ClaudeCitationsConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
}

func (c *ClaudeCitationsConfig) FromMap(m map[string]any) error {
	var err error
	c.Enabled, err = boolPtrValue(m, "enabled")
	return err
}

func (c *ClaudeCitationsConfig) ToMap() (map[string]any, error) {
	out := map[string]any{}
	if c.Enabled != nil {
		out["enabled"] = *c.Enabled
	}
	return out, nil
}

type ClaudeDocumentContent struct {
	ClaudeBaseContent
	Source       ClaudeSource           `json:"source"`
	CacheControl *CacheControl          `json:"cache_control,omitempty"`
	Citations    *ClaudeCitationsConfig `json:"citations,omitempty"`
	Context      string                 `json:"context,omitempty"`
	Title        string                 `json:"title,omitempty"`
}

func (c *ClaudeDocumentContent) UnmarshalJSON(b []byte) error {
	type alias ClaudeDocumentContent
	aux := struct {
		*alias
		Source json.RawMessage `json:"source"`
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	src, err := decodeClaudeSource(aux.Source)
	if err != nil {
		return err
	}
	c.Source = src
	return nil
}

func (c *ClaudeDocumentContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Source, err = decodeClaudeSourceMapField(m, "source")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	if err != nil {
		return err
	}
	c.Citations, err = decodeClaudeCitationsConfigPtrFromMapField(m, "citations")
	if err != nil {
		return err
	}
	c.Context, err = stringValue(m, "context")
	if err != nil {
		return err
	}
	c.Title, err = stringValue(m, "title")
	return err
}

func (c *ClaudeDocumentContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	source, err := claudeSourceToMap(c.Source)
	if err != nil {
		return nil, err
	}
	out["source"] = source
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	if c.Citations != nil {
		citations, err := c.Citations.ToMap()
		if err != nil {
			return nil, err
		}
		out["citations"] = citations
	}
	setMapString(out, "context", c.Context)
	setMapString(out, "title", c.Title)
	return out, nil
}

type ClaudeSearchResultContent struct {
	ClaudeBaseContent
	Source       string                 `json:"source"`
	Title        string                 `json:"title"`
	Content      []ClaudeContent        `json:"content"`
	CacheControl *CacheControl          `json:"cache_control,omitempty"`
	Citations    *ClaudeCitationsConfig `json:"citations,omitempty"`
}

func (c *ClaudeSearchResultContent) UnmarshalJSON(b []byte) error {
	type alias ClaudeSearchResultContent
	aux := struct {
		*alias
		Content []json.RawMessage `json:"content"`
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	items, err := decodeClaudeContentList(aux.Content)
	if err != nil {
		return err
	}
	c.Content = items
	return nil
}

func (c *ClaudeSearchResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Source, err = stringValue(m, "source")
	if err != nil {
		return err
	}
	c.Title, err = stringValue(m, "title")
	if err != nil {
		return err
	}
	c.Content, err = decodeClaudeContentListMapField(m, "content")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	if err != nil {
		return err
	}
	c.Citations, err = decodeClaudeCitationsConfigPtrFromMapField(m, "citations")
	return err
}

func (c *ClaudeSearchResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["source"] = c.Source
	out["title"] = c.Title
	content, err := claudeContentListToMaps(c.Content)
	if err != nil {
		return nil, err
	}
	out["content"] = content
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	if c.Citations != nil {
		citations, err := c.Citations.ToMap()
		if err != nil {
			return nil, err
		}
		out["citations"] = citations
	}
	return out, nil
}

type ClaudeThinkingContent struct {
	ClaudeBaseContent
	Signature string `json:"signature"`
	Thinking  string `json:"thinking"`
}

func (c *ClaudeThinkingContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Signature, err = stringValue(m, "signature")
	if err != nil {
		return err
	}
	c.Thinking, err = stringValue(m, "thinking")
	return err
}

func (c *ClaudeThinkingContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["signature"] = c.Signature
	out["thinking"] = c.Thinking
	return out, nil
}

type ClaudeRedactedThinkingContent struct {
	ClaudeBaseContent
	Data string `json:"data"`
}

func (c *ClaudeRedactedThinkingContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Data, err = stringValue(m, "data")
	return err
}

func (c *ClaudeRedactedThinkingContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["data"] = c.Data
	return out, nil
}

type ClaudeCaller struct {
	Type   string `json:"type"`
	ToolId string `json:"tool_id,omitempty"`
}

func (c *ClaudeCaller) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	c.ToolId, err = stringValue(m, "tool_id")
	return err
}

func (c *ClaudeCaller) ToMap() (map[string]any, error) {
	out := map[string]any{"type": c.Type}
	setMapString(out, "tool_id", c.ToolId)
	return out, nil
}

type ClaudeToolUseContent struct {
	ClaudeBaseContent
	Id           string         `json:"id"`
	Input        map[string]any `json:"input"`
	Name         string         `json:"name"`
	CacheControl *CacheControl  `json:"cache_control,omitempty"`
	Caller       *ClaudeCaller  `json:"caller,omitempty"`
}

func (c *ClaudeToolUseContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Id, err = stringValue(m, "id")
	if err != nil {
		return err
	}
	c.Input, err = mapStringAnyValue(m, "input")
	if err != nil {
		return err
	}
	c.Name, err = stringValue(m, "name")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	if err != nil {
		return err
	}
	c.Caller, err = decodeClaudeCallerPtrFromMapField(m, "caller")
	return err
}

func (c *ClaudeToolUseContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["id"] = c.Id
	if c.Input != nil {
		out["input"] = c.Input
	}
	out["name"] = c.Name
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	if c.Caller != nil {
		caller, err := c.Caller.ToMap()
		if err != nil {
			return nil, err
		}
		out["caller"] = caller
	}
	return out, nil
}

type ClaudeToolResultContent struct {
	ClaudeBaseContent
	ToolUseId    string        `json:"tool_use_id"`
	Content      any           `json:"content,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
	IsError      *bool         `json:"is_error,omitempty"`
}

func (c *ClaudeToolResultContent) UnmarshalJSON(b []byte) error {
	type alias ClaudeToolResultContent
	aux := struct {
		*alias
		Content json.RawMessage `json:"content"`
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	c.Content = nil
	raw := bytes.TrimSpace(aux.Content)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		c.Content = s
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return err
	}
	contents, err := decodeClaudeContentList(items)
	if err != nil {
		return err
	}
	c.Content = contents
	return nil
}

func (c *ClaudeToolResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.ToolUseId, err = stringValue(m, "tool_use_id")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	if err != nil {
		return err
	}
	c.IsError, err = boolPtrValue(m, "is_error")
	if err != nil {
		return err
	}
	content, err := decodeToolResultContentFromMap(m["content"])
	if err != nil {
		return err
	}
	c.Content = content
	return nil
}
func (c *ClaudeToolResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["tool_use_id"] = c.ToolUseId
	content, err := toolResultContentToAny(c.Content)
	if err != nil {
		return nil, err
	}
	if content != nil {
		out["content"] = content
	}
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	if c.IsError != nil {
		out["is_error"] = *c.IsError
	}
	return out, nil
}

type ClaudeToolReferenceContent struct {
	ClaudeBaseContent
	ToolName     string        `json:"tool_name"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

func (c *ClaudeToolReferenceContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.ToolName, err = stringValue(m, "tool_name")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	return err
}

func (c *ClaudeToolReferenceContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["tool_name"] = c.ToolName
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	return out, nil
}

type ClaudeServerToolUseContent struct {
	ClaudeToolUseContent
}

func (c *ClaudeServerToolUseContent) FromMap(m map[string]any) error {
	return c.ClaudeToolUseContent.FromMap(m)
}
func (c *ClaudeServerToolUseContent) ToMap() (map[string]any, error) {
	return c.ClaudeToolUseContent.ToMap()
}

type ClaudeWebSearchResultContent struct {
	ClaudeBaseContent
	EncryptedContent string `json:"encrypted_content"`
	Title            string `json:"title"`
	URL              string `json:"url"`
	PageAge          string `json:"page_age,omitempty"`
}

func (c *ClaudeWebSearchResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.EncryptedContent, err = stringValue(m, "encrypted_content")
	if err != nil {
		return err
	}
	c.Title, err = stringValue(m, "title")
	if err != nil {
		return err
	}
	c.URL, err = stringValue(m, "url")
	if err != nil {
		return err
	}
	c.PageAge, err = stringValue(m, "page_age")
	return err
}

func (c *ClaudeWebSearchResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["encrypted_content"] = c.EncryptedContent
	out["title"] = c.Title
	out["url"] = c.URL
	setMapString(out, "page_age", c.PageAge)
	return out, nil
}

type ClaudeWebSearchToolRequestErrorContent struct {
	ClaudeBaseContent
	ErrorCode string `json:"error_code"`
}

func (c *ClaudeWebSearchToolRequestErrorContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.ErrorCode, err = stringValue(m, "error_code")
	return err
}
func (c *ClaudeWebSearchToolRequestErrorContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["error_code"] = c.ErrorCode
	return out, nil
}

type ClaudeWebSearchToolResultContent struct {
	ClaudeBaseContent
	ToolUseId    string        `json:"tool_use_id"`
	Content      any           `json:"content,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
	Caller       *ClaudeCaller `json:"caller,omitempty"`
}

func (c *ClaudeWebSearchToolResultContent) UnmarshalJSON(b []byte) error {
	type alias ClaudeWebSearchToolResultContent
	aux := struct {
		*alias
		Content json.RawMessage `json:"content"`
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	raw := bytes.TrimSpace(aux.Content)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		c.Content = nil
		return nil
	}
	if raw[0] == '[' {
		var list []ClaudeWebSearchResultContent
		if err := json.Unmarshal(raw, &list); err != nil {
			return err
		}
		c.Content = list
		return nil
	}
	var errContent ClaudeWebSearchToolRequestErrorContent
	if err := json.Unmarshal(raw, &errContent); err != nil {
		return err
	}
	c.Content = &errContent
	return nil
}

func (c *ClaudeWebSearchToolResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.ToolUseId, err = stringValue(m, "tool_use_id")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	if err != nil {
		return err
	}
	c.Caller, err = decodeClaudeCallerPtrFromMapField(m, "caller")
	if err != nil {
		return err
	}
	content, err := decodeWebSearchToolResultFromMap(m["content"])
	if err != nil {
		return err
	}
	c.Content = content
	return nil
}
func (c *ClaudeWebSearchToolResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["tool_use_id"] = c.ToolUseId
	content, err := webSearchToolResultToAny(c.Content)
	if err != nil {
		return nil, err
	}
	if content != nil {
		out["content"] = content
	}
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	if c.Caller != nil {
		caller, err := c.Caller.ToMap()
		if err != nil {
			return nil, err
		}
		out["caller"] = caller
	}
	return out, nil
}

type ClaudeWebFetchToolResultErrorContent struct {
	ClaudeBaseContent
	ErrorCode string `json:"error_code"`
}

func (c *ClaudeWebFetchToolResultErrorContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.ErrorCode, err = stringValue(m, "error_code")
	return err
}
func (c *ClaudeWebFetchToolResultErrorContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["error_code"] = c.ErrorCode
	return out, nil
}

type ClaudeWebFetchResultContent struct {
	ClaudeBaseContent
	URL         string        `json:"url"`
	RetrievedAt string        `json:"retrieved_at,omitempty"`
	Content     ClaudeContent `json:"content"`
}

func (c *ClaudeWebFetchResultContent) UnmarshalJSON(b []byte) error {
	type alias ClaudeWebFetchResultContent
	aux := struct {
		*alias
		Content json.RawMessage `json:"content"`
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	content, err := decodeClaudeContent(aux.Content)
	if err != nil {
		return err
	}
	c.Content = content
	return nil
}

func (c *ClaudeWebFetchResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.URL, err = stringValue(m, "url")
	if err != nil {
		return err
	}
	c.RetrievedAt, err = stringValue(m, "retrieved_at")
	if err != nil {
		return err
	}
	c.Content, err = decodeClaudeContentMapField(m)
	return err
}

func (c *ClaudeWebFetchResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["url"] = c.URL
	setMapString(out, "retrieved_at", c.RetrievedAt)
	content, err := claudeContentToMap(c.Content)
	if err != nil {
		return nil, err
	}
	out["content"] = content
	return out, nil
}

type ClaudeWebFetchToolResultContent struct {
	ClaudeBaseContent
	Content      ClaudeContent `json:"content"`
	ToolUseId    string        `json:"tool_user_id"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
	Caller       *ClaudeCaller `json:"caller,omitempty"`
}

func (c *ClaudeWebFetchToolResultContent) UnmarshalJSON(b []byte) error {
	type alias ClaudeWebFetchToolResultContent
	aux := struct {
		*alias
		Content json.RawMessage `json:"content"`
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	content, err := decodeClaudeContent(aux.Content)
	if err != nil {
		return err
	}
	c.Content = content
	return nil
}

func (c *ClaudeWebFetchToolResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Content, err = decodeClaudeContentMapField(m)
	if err != nil {
		return err
	}
	c.ToolUseId, err = stringValue(m, "tool_user_id")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	if err != nil {
		return err
	}
	c.Caller, err = decodeClaudeCallerPtrFromMapField(m, "caller")
	return err
}

func (c *ClaudeWebFetchToolResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	content, err := claudeContentToMap(c.Content)
	if err != nil {
		return nil, err
	}
	out["content"] = content
	out["tool_user_id"] = c.ToolUseId
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	if c.Caller != nil {
		caller, err := c.Caller.ToMap()
		if err != nil {
			return nil, err
		}
		out["caller"] = caller
	}
	return out, nil
}

type ClaudeCodeExecutionToolResultErrorContent struct {
	ClaudeBaseContent
	ErrorCode string `json:"error_code"`
}

func (c *ClaudeCodeExecutionToolResultErrorContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.ErrorCode, err = stringValue(m, "error_code")
	return err
}
func (c *ClaudeCodeExecutionToolResultErrorContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["error_code"] = c.ErrorCode
	return out, nil
}

type ClaudeCodeExecutionOutputContent struct {
	ClaudeBaseContent
	FileId string `json:"file_id"`
}

func (c *ClaudeCodeExecutionOutputContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.FileId, err = stringValue(m, "file_id")
	return err
}

func (c *ClaudeCodeExecutionOutputContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["file_id"] = c.FileId
	return out, nil
}

type ClaudeCodeExecutionResultContent struct {
	ClaudeBaseContent
	Content    []ClaudeCodeExecutionOutputContent `json:"content"`
	ReturnCode int                                `json:"return_code"`
	Stderr     string                             `json:"stderr"`
	Stdout     string                             `json:"stdout"`
}

func (c *ClaudeCodeExecutionResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Content, err = decodeClaudeCodeExecutionOutputListFromMapField(m, "content")
	if err != nil {
		return err
	}
	c.ReturnCode, err = intValue(m, "return_code")
	if err != nil {
		return err
	}
	c.Stderr, err = stringValue(m, "stderr")
	if err != nil {
		return err
	}
	c.Stdout, err = stringValue(m, "stdout")
	return err
}

func (c *ClaudeCodeExecutionResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	content, err := claudeCodeExecutionOutputListToMaps(c.Content)
	if err != nil {
		return nil, err
	}
	out["content"] = content
	out["return_code"] = c.ReturnCode
	out["stderr"] = c.Stderr
	out["stdout"] = c.Stdout
	return out, nil
}

type ClaudeEncryptedCodeExecutionResultContent struct {
	ClaudeBaseContent
	Content         []ClaudeCodeExecutionOutputContent `json:"content"`
	ReturnCode      int                                `json:"return_code"`
	Stderr          string                             `json:"stderr"`
	EncryptedStdout string                             `json:"encrypted_stdout"`
}

func (c *ClaudeEncryptedCodeExecutionResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Content, err = decodeClaudeCodeExecutionOutputListFromMapField(m, "content")
	if err != nil {
		return err
	}
	c.ReturnCode, err = intValue(m, "return_code")
	if err != nil {
		return err
	}
	c.Stderr, err = stringValue(m, "stderr")
	if err != nil {
		return err
	}
	c.EncryptedStdout, err = stringValue(m, "encrypted_stdout")
	return err
}
func (c *ClaudeEncryptedCodeExecutionResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	content, err := claudeCodeExecutionOutputListToMaps(c.Content)
	if err != nil {
		return nil, err
	}
	out["content"] = content
	out["return_code"] = c.ReturnCode
	out["stderr"] = c.Stderr
	out["encrypted_stdout"] = c.EncryptedStdout
	return out, nil
}

type ClaudeCodeExecutionToolResultContent struct {
	ClaudeBaseContent
	ToolUseId    string        `json:"tool_user_id"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
	Content      ClaudeContent `json:"content"`
}

func (c *ClaudeCodeExecutionToolResultContent) UnmarshalJSON(b []byte) error {
	type alias ClaudeCodeExecutionToolResultContent
	aux := struct {
		*alias
		Content json.RawMessage `json:"content"`
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	content, err := decodeClaudeContent(aux.Content)
	if err != nil {
		return err
	}
	c.Content = content
	return nil
}

func (c *ClaudeCodeExecutionToolResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.ToolUseId, err = stringValue(m, "tool_user_id")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	if err != nil {
		return err
	}
	c.Content, err = decodeClaudeContentMapField(m)
	return err
}
func (c *ClaudeCodeExecutionToolResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["tool_user_id"] = c.ToolUseId
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	content, err := claudeContentToMap(c.Content)
	if err != nil {
		return nil, err
	}
	out["content"] = content
	return out, nil
}

type ClaudeBashCodeExecutionToolResultErrorContent struct {
	ClaudeBaseContent
	ErrorCode string `json:"error_code"`
}

func (c *ClaudeBashCodeExecutionToolResultErrorContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.ErrorCode, err = stringValue(m, "error_code")
	return err
}
func (c *ClaudeBashCodeExecutionToolResultErrorContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["error_code"] = c.ErrorCode
	return out, nil
}

type ClaudeBashCodeExecutionOutputContent struct {
	ClaudeBaseContent
	FileId string `json:"file_id"`
}

func (c *ClaudeBashCodeExecutionOutputContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.FileId, err = stringValue(m, "file_id")
	return err
}
func (c *ClaudeBashCodeExecutionOutputContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["file_id"] = c.FileId
	return out, nil
}

type ClaudeBashCodeExecutionResultContent struct {
	ClaudeBaseContent
	Content    []ClaudeBashCodeExecutionOutputContent `json:"content"`
	ReturnCode int                                    `json:"return_code"`
	Stderr     string                                 `json:"stderr"`
	Stdout     string                                 `json:"stdout"`
}

func (c *ClaudeBashCodeExecutionResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Content, err = decodeClaudeBashCodeExecutionOutputListFromMapField(m, "content")
	if err != nil {
		return err
	}
	c.ReturnCode, err = intValue(m, "return_code")
	if err != nil {
		return err
	}
	c.Stderr, err = stringValue(m, "stderr")
	if err != nil {
		return err
	}
	c.Stdout, err = stringValue(m, "stdout")
	return err
}
func (c *ClaudeBashCodeExecutionResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	content, err := claudeBashCodeExecutionOutputListToMaps(c.Content)
	if err != nil {
		return nil, err
	}
	out["content"] = content
	out["return_code"] = c.ReturnCode
	out["stderr"] = c.Stderr
	out["stdout"] = c.Stdout
	return out, nil
}

type ClaudeBashCodeExecutionToolResultContent struct {
	ClaudeBaseContent
	ToolUseId    string        `json:"tool_user_id"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
	Content      ClaudeContent `json:"content"`
}

func (c *ClaudeBashCodeExecutionToolResultContent) UnmarshalJSON(b []byte) error {
	type alias ClaudeBashCodeExecutionToolResultContent
	aux := struct {
		*alias
		Content json.RawMessage `json:"content"`
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	content, err := decodeClaudeContent(aux.Content)
	if err != nil {
		return err
	}
	c.Content = content
	return nil
}

func (c *ClaudeBashCodeExecutionToolResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.ToolUseId, err = stringValue(m, "tool_user_id")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	if err != nil {
		return err
	}
	c.Content, err = decodeClaudeContentMapField(m)
	return err
}
func (c *ClaudeBashCodeExecutionToolResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["tool_user_id"] = c.ToolUseId
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	content, err := claudeContentToMap(c.Content)
	if err != nil {
		return nil, err
	}
	out["content"] = content
	return out, nil
}

type ClaudeTextEditorCodeExecutionResultErrorContent struct {
	ClaudeBaseContent
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message,omitempty"`
}

func (c *ClaudeTextEditorCodeExecutionResultErrorContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.ErrorCode, err = stringValue(m, "error_code")
	if err != nil {
		return err
	}
	c.ErrorMessage, err = stringValue(m, "error_message")
	return err
}
func (c *ClaudeTextEditorCodeExecutionResultErrorContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["error_code"] = c.ErrorCode
	setMapString(out, "error_message", c.ErrorMessage)
	return out, nil
}

type ClaudeTextEditorCodeExecutionViewResultContent struct {
	ClaudeBaseContent
	Content    string `json:"content"`
	FileType   string `json:"file_type"`
	NumLines   *int   `json:"num_lines,omitempty"`
	StartLine  *int   `json:"start_line,omitempty"`
	TotalLines *int   `json:"total_lines,omitempty"`
}

func (c *ClaudeTextEditorCodeExecutionViewResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Content, err = stringValue(m, "content")
	if err != nil {
		return err
	}
	c.FileType, err = stringValue(m, "file_type")
	if err != nil {
		return err
	}
	c.NumLines, err = intPtrValue(m, "num_lines")
	if err != nil {
		return err
	}
	c.StartLine, err = intPtrValue(m, "start_line")
	if err != nil {
		return err
	}
	c.TotalLines, err = intPtrValue(m, "total_lines")
	return err
}
func (c *ClaudeTextEditorCodeExecutionViewResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["content"] = c.Content
	out["file_type"] = c.FileType
	if c.NumLines != nil {
		out["num_lines"] = *c.NumLines
	}
	if c.StartLine != nil {
		out["start_line"] = *c.StartLine
	}
	if c.TotalLines != nil {
		out["total_lines"] = *c.TotalLines
	}
	return out, nil
}

type ClaudeTextEditorCodeExecutionCreateResultContent struct {
	ClaudeBaseContent
	IsFileUpdate bool `json:"is_file_update"`
}

func (c *ClaudeTextEditorCodeExecutionCreateResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.IsFileUpdate, err = boolValue(m, "is_file_update")
	return err
}
func (c *ClaudeTextEditorCodeExecutionCreateResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	if c.IsFileUpdate {
		out["is_file_update"] = c.IsFileUpdate
	}
	return out, nil
}

type ClaudeTextEditorCodeExecutionStrReplaceResultContent struct {
	ClaudeBaseContent
	Lines    []string `json:"lines,omitempty"`
	NewLines *int     `json:"new_lines,omitempty"`
	NewStart *int     `json:"new_start,omitempty"`
	OldLines *int     `json:"old_lines,omitempty"`
	OldStart *int     `json:"old_start,omitempty"`
}

func (c *ClaudeTextEditorCodeExecutionStrReplaceResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Lines, err = stringSliceValue(m, "lines")
	if err != nil {
		return err
	}
	c.NewLines, err = intPtrValue(m, "new_lines")
	if err != nil {
		return err
	}
	c.NewStart, err = intPtrValue(m, "new_start")
	if err != nil {
		return err
	}
	c.OldLines, err = intPtrValue(m, "old_lines")
	if err != nil {
		return err
	}
	c.OldStart, err = intPtrValue(m, "old_start")
	return err
}
func (c *ClaudeTextEditorCodeExecutionStrReplaceResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	setMapStringSlice(out, "lines", c.Lines)
	if c.NewLines != nil {
		out["new_lines"] = *c.NewLines
	}
	if c.NewStart != nil {
		out["new_start"] = *c.NewStart
	}
	if c.OldLines != nil {
		out["old_lines"] = *c.OldLines
	}
	if c.OldStart != nil {
		out["old_start"] = *c.OldStart
	}
	return out, nil
}

type ClaudeTextEditorCodeExecutionToolResultContent struct {
	ClaudeBaseContent
	Content      ClaudeContent `json:"content"`
	ToolUseId    string        `json:"tool_user_id"`
	CacheControl *CacheControl `json:"cache_control"`
}

func (c *ClaudeTextEditorCodeExecutionToolResultContent) UnmarshalJSON(b []byte) error {
	type alias ClaudeTextEditorCodeExecutionToolResultContent
	aux := struct {
		*alias
		Content json.RawMessage `json:"content"`
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	content, err := decodeClaudeContent(aux.Content)
	if err != nil {
		return err
	}
	c.Content = content
	return nil
}

func (c *ClaudeTextEditorCodeExecutionToolResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Content, err = decodeClaudeContentMapField(m)
	if err != nil {
		return err
	}
	c.ToolUseId, err = stringValue(m, "tool_user_id")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	return err
}
func (c *ClaudeTextEditorCodeExecutionToolResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	content, err := claudeContentToMap(c.Content)
	if err != nil {
		return nil, err
	}
	out["content"] = content
	out["tool_user_id"] = c.ToolUseId
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	return out, nil
}

type ClaudeToolSearchToolResultErrorContent struct {
	ClaudeBaseContent
	ErrorCode string `json:"error_code"`
}

func (c *ClaudeToolSearchToolResultErrorContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.ErrorCode, err = stringValue(m, "error_code")
	return err
}
func (c *ClaudeToolSearchToolResultErrorContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["error_code"] = c.ErrorCode
	return out, nil
}

type ClaudeToolSearchToolSearchResultContent struct {
	ClaudeBaseContent
	ToolReferences []ClaudeToolReferenceContent `json:"tool_references"`
}

func (c *ClaudeToolSearchToolSearchResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.ToolReferences, err = decodeClaudeToolReferenceListFromMapField(m, "tool_references")
	return err
}
func (c *ClaudeToolSearchToolSearchResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	refs, err := claudeToolReferenceListToMaps(c.ToolReferences)
	if err != nil {
		return nil, err
	}
	out["tool_references"] = refs
	return out, nil
}

type ClaudeToolSearchToolResultContent struct {
	ClaudeBaseContent
	Content      ClaudeContent `json:"content"`
	ToolUseId    string        `json:"tool_user_id"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

func (c *ClaudeToolSearchToolResultContent) UnmarshalJSON(b []byte) error {
	type alias ClaudeToolSearchToolResultContent
	aux := struct {
		*alias
		Content json.RawMessage `json:"content"`
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	content, err := decodeClaudeContent(aux.Content)
	if err != nil {
		return err
	}
	c.Content = content
	return nil
}

func (c *ClaudeToolSearchToolResultContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Content, err = decodeClaudeContentMapField(m)
	if err != nil {
		return err
	}
	c.ToolUseId, err = stringValue(m, "tool_user_id")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	return err
}

func (c *ClaudeToolSearchToolResultContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	content, err := claudeContentToMap(c.Content)
	if err != nil {
		return nil, err
	}
	out["content"] = content
	out["tool_user_id"] = c.ToolUseId
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	return out, nil
}

type ClaudeContainerUploadContent struct {
	ClaudeBaseContent
	FileId       string        `json:"file_id"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

func (c *ClaudeContainerUploadContent) FromMap(m map[string]any) error {
	if err := c.ClaudeBaseContent.FromMap(m); err != nil {
		return err
	}
	var err error
	c.FileId, err = stringValue(m, "file_id")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	return err
}

func (c *ClaudeContainerUploadContent) ToMap() (map[string]any, error) {
	out, _ := c.ClaudeBaseContent.ToMap()
	out["file_id"] = c.FileId
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	return out, nil
}

type ClaudeUnknownContent struct {
	ClaudeBaseContent                 // carries the raw type string for any unrecognized content type
	Raw               json.RawMessage `json:"-"`
}

func (c *ClaudeUnknownContent) MarshalJSON() ([]byte, error) {
	if len(c.Raw) > 0 {
		return c.Raw, nil
	}
	return json.Marshal(c.ClaudeBaseContent)
}

type ClaudeContent interface {
	GetType() string
}

type ClaudeMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func (c *ClaudeMessage) UnmarshalJSON(b []byte) error {
	type wire struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	c.Role = w.Role
	c.Content = nil
	raw := bytes.TrimSpace(w.Content)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		c.Content = s
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return err
	}
	contents, err := decodeClaudeContentList(items)
	if err != nil {
		return err
	}
	c.Content = contents
	return nil
}

func (c *ClaudeMessage) FromMap(m map[string]any) error {
	role, ok := m["role"].(string)
	if !ok {
		return fmt.Errorf("message role must be string")
	}
	c.Role = role
	content, err := decodeClaudeMessageContentFromMap(m["content"])
	if err != nil {
		return err
	}
	c.Content = content
	return nil
}
func (c *ClaudeMessage) ToMap() (map[string]any, error) {
	out := map[string]any{"role": c.Role}
	content, err := claudeMessageContentToAny(c.Content)
	if err != nil {
		return nil, err
	}
	out["content"] = content
	return out, nil
}

type ClaudeUserLocation struct {
	Type     string `json:"type"`
	City     string `json:"city,omitempty"`
	Country  string `json:"country,omitempty"`
	Region   string `json:"region,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

func (c *ClaudeUserLocation) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	c.City, err = stringValue(m, "city")
	if err != nil {
		return err
	}
	c.Country, err = stringValue(m, "country")
	if err != nil {
		return err
	}
	c.Region, err = stringValue(m, "region")
	if err != nil {
		return err
	}
	c.Timezone, err = stringValue(m, "timezone")
	return err
}

func (c *ClaudeUserLocation) ToMap() (map[string]any, error) {
	out := map[string]any{"type": c.Type}
	setMapString(out, "city", c.City)
	setMapString(out, "country", c.Country)
	setMapString(out, "region", c.Region)
	setMapString(out, "timezone", c.Timezone)
	return out, nil
}

type ClaudeTool struct {
	Name                string                 `json:"name"`
	Description         string                 `json:"description,omitempty"`
	InputSchema         *ClaudeInputSchema     `json:"input_schema,omitempty"`
	AllowedCallers      []string               `json:"allowed_callers,omitempty"`
	CacheControl        *CacheControl          `json:"cache_control,omitempty"`
	DeferLoading        bool                   `json:"defer_loading,omitempty"`
	EagerInputStreaming bool                   `json:"eager_input_streaming,omitempty"`
	InputExamples       []map[string]any       `json:"input_examples,omitempty"`
	Strict              bool                   `json:"strict,omitempty"`
	Type                string                 `json:"type,omitempty"`
	MaxCharacters       int                    `json:"max_characters,omitempty"`
	AllowedDomains      []string               `json:"allowed_domains,omitempty"`
	BlockedDomains      []string               `json:"blocked_domains,omitempty"`
	MaxUses             int                    `json:"max_uses,omitempty"`
	UserLocation        *ClaudeUserLocation    `json:"user_location,omitempty"`
	MaxContentTokens    int                    `json:"max_content_tokens,omitempty"`
	Citations           *ClaudeCitationsConfig `json:"citations,omitempty"`
	UseCache            *bool                  `json:"user_cache,omitempty"`
}

func (c *ClaudeTool) FromMap(m map[string]any) error {
	var err error
	c.Name, err = stringValue(m, "name")
	if err != nil {
		return err
	}
	c.Description, err = stringValue(m, "description")
	if err != nil {
		return err
	}
	c.InputSchema, err = decodeClaudeInputSchemaPtrFromMapField(m, "input_schema")
	if err != nil {
		return err
	}
	c.AllowedCallers, err = stringSliceValue(m, "allowed_callers")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	if err != nil {
		return err
	}
	c.DeferLoading, err = boolValue(m, "defer_loading")
	if err != nil {
		return err
	}
	c.EagerInputStreaming, err = boolValue(m, "eager_input_streaming")
	if err != nil {
		return err
	}
	c.InputExamples, err = mapStringAnySliceValue(m, "input_examples")
	if err != nil {
		return err
	}
	c.Strict, err = boolValue(m, "strict")
	if err != nil {
		return err
	}
	c.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	c.MaxCharacters, err = intValue(m, "max_characters")
	if err != nil {
		return err
	}
	c.AllowedDomains, err = stringSliceValue(m, "allowed_domains")
	if err != nil {
		return err
	}
	c.BlockedDomains, err = stringSliceValue(m, "blocked_domains")
	if err != nil {
		return err
	}
	c.MaxUses, err = intValue(m, "max_uses")
	if err != nil {
		return err
	}
	c.UserLocation, err = decodeClaudeUserLocationPtrFromMapField(m, "user_location")
	if err != nil {
		return err
	}
	c.MaxContentTokens, err = intValue(m, "max_content_tokens")
	if err != nil {
		return err
	}
	c.Citations, err = decodeClaudeCitationsConfigPtrFromMapField(m, "citations")
	if err != nil {
		return err
	}
	c.UseCache, err = boolPtrValue(m, "user_cache")
	return err
}

func (c *ClaudeTool) ToMap() (map[string]any, error) {
	out := map[string]any{"name": c.Name}
	setMapString(out, "description", c.Description)
	if c.InputSchema != nil {
		inputSchema, err := c.InputSchema.ToMap()
		if err != nil {
			return nil, err
		}
		out["input_schema"] = inputSchema
	}
	setMapStringSlice(out, "allowed_callers", c.AllowedCallers)
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	setMapBool(out, "defer_loading", c.DeferLoading)
	setMapBool(out, "eager_input_streaming", c.EagerInputStreaming)
	if len(c.InputExamples) > 0 {
		items := make([]any, 0, len(c.InputExamples))
		for _, item := range c.InputExamples {
			items = append(items, item)
		}
		out["input_examples"] = items
	}
	setMapBool(out, "strict", c.Strict)
	setMapString(out, "type", c.Type)
	setMapInt(out, "max_characters", c.MaxCharacters)
	setMapStringSlice(out, "allowed_domains", c.AllowedDomains)
	setMapStringSlice(out, "blocked_domains", c.BlockedDomains)
	setMapInt(out, "max_uses", c.MaxUses)
	if c.UserLocation != nil {
		userLocation, err := c.UserLocation.ToMap()
		if err != nil {
			return nil, err
		}
		out["user_location"] = userLocation
	}
	setMapInt(out, "max_content_tokens", c.MaxContentTokens)
	if c.Citations != nil {
		citations, err := c.Citations.ToMap()
		if err != nil {
			return nil, err
		}
		out["citations"] = citations
	}
	if c.UseCache != nil {
		out["user_cache"] = *c.UseCache
	}
	return out, nil
}

type ClaudeInputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required,omitempty"`
}

func (c *ClaudeInputSchema) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	c.Properties, err = mapStringAnyValue(m, "properties")
	if err != nil {
		return err
	}
	c.Required, err = stringSliceValue(m, "required")
	return err
}

func (c *ClaudeInputSchema) ToMap() (map[string]any, error) {
	out := map[string]any{
		"type":       c.Type,
		"properties": c.Properties,
	}
	setMapStringSlice(out, "required", c.Required)
	return out, nil
}

type ClaudeJsonOutputFormat struct {
	Schema map[string]any `json:"schema"`
	Type   string         `json:"type"`
}

func (c *ClaudeJsonOutputFormat) FromMap(m map[string]any) error {
	var err error
	c.Schema, err = mapStringAnyValue(m, "schema")
	if err != nil {
		return err
	}
	c.Type, err = stringValue(m, "type")
	return err
}

func (c *ClaudeJsonOutputFormat) ToMap() (map[string]any, error) {
	return map[string]any{
		"schema": c.Schema,
		"type":   c.Type,
	}, nil
}

type ClaudeOutputConfig struct {
	Effort string                  `json:"effort,omitempty"`
	Format *ClaudeJsonOutputFormat `json:"format,omitempty"`
}

func (c *ClaudeOutputConfig) FromMap(m map[string]any) error {
	var err error
	c.Effort, err = stringValue(m, "effort")
	if err != nil {
		return err
	}
	c.Format, err = decodeClaudeJsonOutputFormatPtrFromMapField(m, "format")
	return err
}

func (c *ClaudeOutputConfig) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "effort", c.Effort)
	if c.Format != nil {
		format, err := c.Format.ToMap()
		if err != nil {
			return nil, err
		}
		out["format"] = format
	}
	return out, nil
}

type BaseThinkingConfig struct {
	Type string `json:"type"`
}

func (c *BaseThinkingConfig) GetType() string { return c.Type }

func (c *BaseThinkingConfig) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	return err
}

func (c *BaseThinkingConfig) ToMap() (map[string]any, error) {
	return map[string]any{"type": c.Type}, nil
}

type ThinkingConfigEnabled struct {
	BaseThinkingConfig
	BudgetTokens int    `json:"budget_tokens"`
	Display      string `json:"display,omitempty"`
}

func (c *ThinkingConfigEnabled) FromMap(m map[string]any) error {
	if err := c.BaseThinkingConfig.FromMap(m); err != nil {
		return err
	}
	var err error
	c.BudgetTokens, err = intValue(m, "budget_tokens")
	if err != nil {
		return err
	}
	c.Display, err = stringValue(m, "display")
	return err
}

func (c *ThinkingConfigEnabled) ToMap() (map[string]any, error) {
	out, _ := c.BaseThinkingConfig.ToMap()
	out["budget_tokens"] = c.BudgetTokens
	setMapString(out, "display", c.Display)
	return out, nil
}

type ThinkingConfigAdaptive struct {
	BaseThinkingConfig
	Display string `json:"display,omitempty"`
}

func (c *ThinkingConfigAdaptive) FromMap(m map[string]any) error {
	if err := c.BaseThinkingConfig.FromMap(m); err != nil {
		return err
	}
	var err error
	c.Display, err = stringValue(m, "display")
	return err
}

func (c *ThinkingConfigAdaptive) ToMap() (map[string]any, error) {
	out, _ := c.BaseThinkingConfig.ToMap()
	setMapString(out, "display", c.Display)
	return out, nil
}

type ThinkingConfigDisabled struct {
	BaseThinkingConfig
}

func (c *ThinkingConfigDisabled) FromMap(m map[string]any) error {
	return c.BaseThinkingConfig.FromMap(m)
}
func (c *ThinkingConfigDisabled) ToMap() (map[string]any, error) { return c.BaseThinkingConfig.ToMap() }

type ThinkingConfigUnknown struct {
	BaseThinkingConfig                 // carries the raw type string for any unrecognized thinking config type
	Raw                json.RawMessage `json:"-"`
}

func (t *ThinkingConfigUnknown) MarshalJSON() ([]byte, error) {
	if len(t.Raw) > 0 {
		return t.Raw, nil
	}
	return json.Marshal(t.BaseThinkingConfig)
}

type ThinkingConfigInterface interface {
	GetType() string
}

type ThinkingConfig struct {
	Data ThinkingConfigInterface
}

func (c ThinkingConfig) MarshalJSON() ([]byte, error) {
	if c.Data == nil {
		return []byte("null"), nil
	}
	return json.Marshal(c.Data)
}

func (c *ThinkingConfig) UnmarshalJSON(b []byte) error {
	raw := bytes.TrimSpace(b)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		c.Data = nil
		return nil
	}
	data, err := decodeClaudeThinking(raw)
	if err != nil {
		return err
	}
	c.Data = data
	return nil
}

func (c *ThinkingConfig) FromMap(m map[string]any) error {
	data, err := decodeClaudeThinkingFromMap(m)
	if err != nil {
		return err
	}
	c.Data = data
	return nil
}

func (c *ThinkingConfig) ToMap() (map[string]any, error) {
	if c.Data == nil {
		return nil, nil
	}
	switch typed := c.Data.(type) {
	case interface {
		ToMap() (map[string]any, error)
	}:
		return typed.ToMap()
	default:
		return nil, fmt.Errorf("unsupported thinking config type %T", c.Data)
	}
}

type Fallback struct {
	Model     string          `json:"model"`
	MaxTokens *int            `json:"max_tokens,omitempty"`
	Thinking  *ThinkingConfig `json:"thinking,omitempty"`
}

func (f *Fallback) FromMap(m map[string]any) error {
	var err error
	f.Model, err = stringValue(m, "model")
	if err != nil {
		return err
	}
	f.MaxTokens, err = intPtrValue(m, "max_tokens")
	if err != nil {
		return err
	}
	f.Thinking, err = decodeThinkingConfigPtrFromMapField(m, "thinking")
	return err
}

func (f *Fallback) ToMap() (map[string]any, error) {
	out := map[string]any{
		"model": f.Model,
	}
	if f.MaxTokens != nil {
		out["max_tokens"] = *f.MaxTokens
	}
	if f.Thinking != nil {
		thinking, err := f.Thinking.ToMap()
		if err != nil {
			return nil, err
		}
		out["thinking"] = thinking
	}
	return out, nil
}

type ClaudeToolChoice struct {
	Type                   string `json:"type"`
	DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
	Name                   string `json:"name,omitempty"`
}

func (c *ClaudeToolChoice) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	c.DisableParallelToolUse, err = boolValue(m, "disable_parallel_tool_use")
	if err != nil {
		return err
	}
	c.Name, err = stringValue(m, "name")
	return err
}

func (c *ClaudeToolChoice) ToMap() (map[string]any, error) {
	out := map[string]any{"type": c.Type}
	setMapBool(out, "disable_parallel_tool_use", c.DisableParallelToolUse)
	setMapString(out, "name", c.Name)
	return out, nil
}

type ClaudeRequest struct {
	Model               string              `json:"model"`
	Messages            []ClaudeMessage     `json:"messages"`
	System              any                 `json:"system,omitempty"`
	MaxTokens           *int                `json:"max_tokens"`
	CacheControl        *CacheControl       `json:"cache_control,omitempty"`
	Container           string              `json:"container,omitempty"`
	InferenceGeo        string              `json:"inference_geo,omitempty"`
	Fallbacks           []*Fallback         `json:"fallbacks,omitempty"`
	FallbackCreditToken string              `json:"fallback_credit_token,omitempty"`
	OutputConfig        *ClaudeOutputConfig `json:"output_config,omitempty"`
	ServiceTier         string              `json:"service_tier,omitempty"`
	StopSequences       []string            `json:"stop_sequences,omitempty"`
	Stream              *bool               `json:"stream,omitempty"`
	Temperature         *float64            `json:"temperature,omitempty"`
	Thinking            *ThinkingConfig     `json:"thinking,omitempty"`
	TopP                *float64            `json:"top_p,omitempty"`
	TopK                *int                `json:"top_k,omitempty"`
	Tools               []ClaudeTool        `json:"tools,omitempty"`
	ToolChoice          *ClaudeToolChoice   `json:"tool_choice,omitempty"`
	Metadata            *ClaudeMetadata     `json:"metadata,omitempty"`
	AnthropicVersion    string              `json:"anthropic_version,omitempty"`
	AnthropicBeta       []string            `json:"anthropic_beta,omitempty"`
}

func (c *ClaudeRequest) GetPrompt() string {
	var builder strings.Builder
	for i := range c.Messages {
		if c.Messages[i].Role != "user" {
			continue
		}
		switch content := c.Messages[i].Content.(type) {
		case string:
			_, _ = builder.WriteString(content)
		case []ClaudeContent:
			for _, block := range content {
				if block == nil {
					continue
				}
				textBlock, ok := block.(*ClaudeTextContent)
				if !ok || textBlock.Text == "" {
					continue
				}
				if builder.Len() > 0 {
					_, _ = builder.WriteString("\n")
				}
				_, _ = builder.WriteString(textBlock.Text)
			}
		}
	}
	return builder.String()
}

func (c *ClaudeRequest) UnmarshalJSON(b []byte) error {
	type wire struct {
		Model               string              `json:"model"`
		Messages            []ClaudeMessage     `json:"messages"`
		System              json.RawMessage     `json:"system,omitempty"`
		MaxTokens           *int                `json:"max_tokens,omitempty"`
		CacheControl        *CacheControl       `json:"cache_control,omitempty"`
		Container           string              `json:"container,omitempty"`
		InferenceGeo        string              `json:"inference_geo,omitempty"`
		Fallbacks           []*Fallback         `json:"fallbacks,omitempty"`
		FallbackCreditToken string              `json:"fallback_credit_token,omitempty"`
		OutputConfig        *ClaudeOutputConfig `json:"output_config,omitempty"`
		ServiceTier         string              `json:"service_tier,omitempty"`
		StopSequences       []string            `json:"stop_sequences,omitempty"`
		Stream              *bool               `json:"stream,omitempty"`
		Temperature         *float64            `json:"temperature,omitempty"`
		Thinking            *ThinkingConfig     `json:"thinking,omitempty"`
		TopP                *float64            `json:"top_p,omitempty"`
		TopK                *int                `json:"top_k,omitempty"`
		Tools               []ClaudeTool        `json:"tools,omitempty"`
		ToolChoice          *ClaudeToolChoice   `json:"tool_choice,omitempty"`
		Metadata            *ClaudeMetadata     `json:"metadata,omitempty"`
		AnthropicVersion    string              `json:"anthropic_version,omitempty"`
		AnthropicBeta       []string            `json:"anthropic_beta,omitempty"`
	}
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*c = ClaudeRequest{
		Model:               w.Model,
		Messages:            w.Messages,
		MaxTokens:           w.MaxTokens,
		CacheControl:        w.CacheControl,
		Container:           w.Container,
		InferenceGeo:        w.InferenceGeo,
		Fallbacks:           w.Fallbacks,
		FallbackCreditToken: w.FallbackCreditToken,
		OutputConfig:        w.OutputConfig,
		ServiceTier:         w.ServiceTier,
		StopSequences:       w.StopSequences,
		Stream:              w.Stream,
		Temperature:         w.Temperature,
		Thinking:            w.Thinking,
		TopP:                w.TopP,
		TopK:                w.TopK,
		Tools:               w.Tools,
		ToolChoice:          w.ToolChoice,
		Metadata:            w.Metadata,
		AnthropicVersion:    w.AnthropicVersion,
		AnthropicBeta:       w.AnthropicBeta,
	}
	raw := bytes.TrimSpace(w.System)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	if raw[0] == '"' {
		var system string
		if err := json.Unmarshal(raw, &system); err != nil {
			return err
		}
		c.System = system
		return nil
	}
	var rawBlocks []json.RawMessage
	if err := json.Unmarshal(raw, &rawBlocks); err != nil {
		return err
	}
	blocks := make([]ClaudeTextContent, 0, len(rawBlocks))
	for _, item := range rawBlocks {
		content, err := decodeClaudeContent(item)
		if err != nil {
			return err
		}
		textBlock, ok := content.(*ClaudeTextContent)
		if !ok {
			return fmt.Errorf("expected system text block, got %T", content)
		}
		blocks = append(blocks, *textBlock)
	}
	c.System = blocks
	return nil
}

func (c *ClaudeRequest) FromMap(m map[string]any) error {
	var err error
	c.Model, err = stringValue(m, "model")
	if err != nil {
		return err
	}
	c.Messages, err = decodeClaudeMessageListFromMapField(m, "messages")
	if err != nil {
		return err
	}
	c.System, err = decodeClaudeSystemFromMap(m["system"])
	if err != nil {
		return err
	}
	c.MaxTokens, err = intPtrValue(m, "max_tokens")
	if err != nil {
		return err
	}
	c.CacheControl, err = decodeCacheControlPtrFromMapField(m)
	if err != nil {
		return err
	}
	c.Container, err = stringValue(m, "container")
	if err != nil {
		return err
	}
	c.InferenceGeo, err = stringValue(m, "inference_geo")
	if err != nil {
		return err
	}
	c.Fallbacks, err = decodeFallbackListFromMapField(m, "fallbacks")
	if err != nil {
		return err
	}
	c.FallbackCreditToken, err = stringValue(m, "fallback_credit_token")
	if err != nil {
		return err
	}
	c.OutputConfig, err = decodeClaudeOutputConfigPtrFromMapField(m, "output_config")
	if err != nil {
		return err
	}
	c.ServiceTier, err = stringValue(m, "service_tier")
	if err != nil {
		return err
	}
	c.StopSequences, err = stringSliceValue(m, "stop_sequences")
	if err != nil {
		return err
	}
	c.Stream, err = boolPtrValue(m, "stream")
	if err != nil {
		return err
	}
	c.Temperature, err = floatPtrValue(m, "temperature")
	if err != nil {
		return err
	}
	c.Thinking, err = decodeThinkingConfigPtrFromMapField(m, "thinking")
	if err != nil {
		return err
	}
	c.TopP, err = floatPtrValue(m, "top_p")
	if err != nil {
		return err
	}
	c.TopK, err = intPtrValue(m, "top_k")
	if err != nil {
		return err
	}
	c.Tools, err = decodeClaudeToolListFromMapField(m, "tools")
	if err != nil {
		return err
	}
	c.ToolChoice, err = decodeClaudeToolChoicePtrFromMapField(m, "tool_choice")
	if err != nil {
		return err
	}
	c.Metadata, err = decodeClaudeMetadataPtrFromMapField(m, "metadata")
	if err != nil {
		return err
	}
	c.AnthropicVersion, err = stringValue(m, "anthropic_version")
	if err != nil {
		return err
	}
	c.AnthropicBeta, err = stringSliceValue(m, "anthropic_beta")
	return err
}

func (c *ClaudeRequest) ToMap() (map[string]any, error) {
	out := map[string]any{
		"model": c.Model,
	}
	messages, err := claudeMessageListToMaps(c.Messages)
	if err != nil {
		return nil, err
	}
	out["messages"] = messages
	system, err := claudeSystemToAny(c.System)
	if err != nil {
		return nil, err
	}
	if system != nil {
		out["system"] = system
	}
	if c.MaxTokens != nil {
		out["max_tokens"] = *c.MaxTokens
	}
	if c.CacheControl != nil {
		cacheControl, err := c.CacheControl.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_control"] = cacheControl
	}
	setMapString(out, "container", c.Container)
	setMapString(out, "inference_geo", c.InferenceGeo)
	if len(c.Fallbacks) > 0 {
		fallbacks, err := fallbackListToMaps(c.Fallbacks)
		if err != nil {
			return nil, err
		}
		out["fallbacks"] = fallbacks
	}
	setMapString(out, "fallback_credit_token", c.FallbackCreditToken)
	if c.OutputConfig != nil {
		outputConfig, err := c.OutputConfig.ToMap()
		if err != nil {
			return nil, err
		}
		out["output_config"] = outputConfig
	}
	setMapString(out, "service_tier", c.ServiceTier)
	setMapStringSlice(out, "stop_sequences", c.StopSequences)
	if c.Stream != nil {
		out["stream"] = *c.Stream
	}
	if c.Temperature != nil {
		out["temperature"] = *c.Temperature
	}
	if c.Thinking != nil {
		thinking, err := c.Thinking.ToMap()
		if err != nil {
			return nil, err
		}
		out["thinking"] = thinking
	}
	if c.TopP != nil {
		out["top_p"] = *c.TopP
	}
	if c.TopK != nil {
		out["top_k"] = *c.TopK
	}
	if len(c.Tools) > 0 {
		tools, err := claudeToolListToMaps(c.Tools)
		if err != nil {
			return nil, err
		}
		out["tools"] = tools
	}
	if c.ToolChoice != nil {
		toolChoice, err := c.ToolChoice.ToMap()
		if err != nil {
			return nil, err
		}
		out["tool_choice"] = toolChoice
	}
	if c.Metadata != nil {
		metadata, err := c.Metadata.ToMap()
		if err != nil {
			return nil, err
		}
		out["metadata"] = metadata
	}
	setMapString(out, "anthropic_version", c.AnthropicVersion)
	setMapStringSlice(out, "anthropic_beta", c.AnthropicBeta)
	return out, nil
}

type ClaudeServerToolUsage struct {
	WebFetchRequests  int `json:"web_fetch_requests,omitempty"`
	WebSearchRequests int `json:"web_search_requests,omitempty"`
}

func (c *ClaudeServerToolUsage) FromMap(m map[string]any) error {
	var err error
	c.WebFetchRequests, err = intValue(m, "web_fetch_requests")
	if err != nil {
		return err
	}
	c.WebSearchRequests, err = intValue(m, "web_search_requests")
	return err
}

func (c *ClaudeServerToolUsage) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapInt(out, "web_fetch_requests", c.WebFetchRequests)
	setMapInt(out, "web_search_requests", c.WebSearchRequests)
	return out, nil
}

type CacheCreationUsageDetail struct {
	Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens"`
	Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens"`
}

func (c *CacheCreationUsageDetail) FromMap(m map[string]any) error {
	var err error
	c.Ephemeral5mInputTokens, err = intValue(m, "ephemeral_5m_input_tokens")
	if err != nil {
		return err
	}
	c.Ephemeral1hInputTokens, err = intValue(m, "ephemeral_1h_input_tokens")
	return err
}

func (c *CacheCreationUsageDetail) ToMap() (map[string]any, error) {
	return map[string]any{
		"ephemeral_5m_input_tokens": c.Ephemeral5mInputTokens,
		"ephemeral_1h_input_tokens": c.Ephemeral1hInputTokens,
	}, nil
}

type ClaudeUsageByModel struct {
	CacheCreation            *CacheCreationUsageDetail `json:"cache_creation,omitempty"`
	CacheCreationInputTokens int                       `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int                       `json:"cache_read_input_tokens,omitempty"`
	InputTokens              int                       `json:"input_tokens"`
	Model                    string                    `json:"model"`
	OutputTokens             int                       `json:"output_tokens"`
	Type                     string                    `json:"type"`
}

// GetClaudeUsage constructs a ClaudeUsage from the per-model fields, without the Iterations slice.
func (c ClaudeUsageByModel) GetClaudeUsage() *ClaudeUsage {
	return &ClaudeUsage{
		InputTokens:              c.InputTokens,
		OutputTokens:             c.OutputTokens,
		CacheCreationInputTokens: c.CacheCreationInputTokens,
		CacheReadInputTokens:     c.CacheReadInputTokens,
		CacheCreation:            c.CacheCreation,
	}
}

func (c *ClaudeUsageByModel) FromMap(m map[string]any) error {
	var err error
	c.CacheCreation, err = decodeCacheCreationUsageDetailPtrFromMapField(m, "cache_creation")
	if err != nil {
		return err
	}
	c.CacheCreationInputTokens, err = intValue(m, "cache_creation_input_tokens")
	if err != nil {
		return err
	}
	c.CacheReadInputTokens, err = intValue(m, "cache_read_input_tokens")
	if err != nil {
		return err
	}
	c.InputTokens, err = intValue(m, "input_tokens")
	if err != nil {
		return err
	}
	c.Model, err = stringValue(m, "model")
	if err != nil {
		return err
	}
	c.OutputTokens, err = intValue(m, "output_tokens")
	if err != nil {
		return err
	}
	c.Type, err = stringValue(m, "type")
	return err
}

func (c *ClaudeUsageByModel) ToMap() (map[string]any, error) {
	out := map[string]any{
		"input_tokens":  c.InputTokens,
		"model":         c.Model,
		"output_tokens": c.OutputTokens,
		"type":          c.Type,
	}
	if c.CacheCreation != nil {
		cacheCreation, err := c.CacheCreation.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_creation"] = cacheCreation
	}
	setMapInt(out, "cache_creation_input_tokens", c.CacheCreationInputTokens)
	setMapInt(out, "cache_read_input_tokens", c.CacheReadInputTokens)
	return out, nil
}

type ClaudeUsage struct {
	InputTokens              int                       `json:"input_tokens"`
	OutputTokens             int                       `json:"output_tokens"`
	CacheCreationInputTokens int                       `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int                       `json:"cache_read_input_tokens,omitempty"`
	CacheCreation            *CacheCreationUsageDetail `json:"cache_creation,omitempty"`
	InferenceGeo             string                    `json:"inference_geo,omitempty"`
	ServiceTier              string                    `json:"service_tier,omitempty"`
	ServerToolUse            *ClaudeServerToolUsage    `json:"server_tool_use,omitempty"`
	Iterations               []ClaudeUsageByModel      `json:"iterations,omitempty"`
}

// ClaudeStreamMessage models one SSE event payload from Claude /v1/messages streaming.
type ClaudeStreamMessage struct {
	Type         string                    `json:"type"`
	Message      *ClaudeStreamStart        `json:"message,omitempty"`
	Index        int                       `json:"index,omitempty"`
	ContentBlock *ClaudeStreamContentBlock `json:"content_block,omitempty"`
	Delta        *ClaudeStreamDelta        `json:"delta,omitempty"`
	Usage        *ClaudeUsage              `json:"usage,omitempty"`
}

// ClaudeStreamStart contains the initial message metadata in `message_start`.
type ClaudeStreamStart struct {
	ID    string       `json:"id,omitempty"`
	Model string       `json:"model,omitempty"`
	Usage *ClaudeUsage `json:"usage,omitempty"`
}

// ClaudeStreamContentBlock describes a streamed content block header.
type ClaudeStreamContentBlock struct {
	Type string `json:"type,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// ClaudeStreamDelta carries incremental text/tool stop data from Claude SSE.
type ClaudeStreamDelta struct {
	Type        string             `json:"type,omitempty"`
	Text        string             `json:"text,omitempty"`
	PartialJSON string             `json:"partial_json,omitempty"`
	StopReason  string             `json:"stop_reason,omitempty"`
	StopDetails *ClaudeStopDetails `json:"stop_details,omitempty"`
}

func (c *ClaudeUsage) FromMap(m map[string]any) error {
	var err error
	c.InputTokens, err = intValue(m, "input_tokens")
	if err != nil {
		return err
	}
	c.OutputTokens, err = intValue(m, "output_tokens")
	if err != nil {
		return err
	}
	c.CacheCreationInputTokens, err = intValue(m, "cache_creation_input_tokens")
	if err != nil {
		return err
	}
	c.CacheReadInputTokens, err = intValue(m, "cache_read_input_tokens")
	if err != nil {
		return err
	}
	c.CacheCreation, err = decodeCacheCreationUsageDetailPtrFromMapField(m, "cache_creation")
	if err != nil {
		return err
	}
	c.InferenceGeo, err = stringValue(m, "inference_geo")
	if err != nil {
		return err
	}
	c.ServiceTier, err = stringValue(m, "service_tier")
	if err != nil {
		return err
	}
	c.ServerToolUse, err = decodeClaudeServerToolUsagePtrFromMapField(m, "server_tool_use")
	if err != nil {
		return err
	}
	c.Iterations, err = decodeClaudeUsageByModelListFromMapField(m, "iterations")
	return err
}

func (c *ClaudeUsage) ToMap() (map[string]any, error) {
	out := map[string]any{
		"input_tokens":  c.InputTokens,
		"output_tokens": c.OutputTokens,
	}
	setMapInt(out, "cache_creation_input_tokens", c.CacheCreationInputTokens)
	setMapInt(out, "cache_read_input_tokens", c.CacheReadInputTokens)
	if c.CacheCreation != nil {
		cacheCreation, err := c.CacheCreation.ToMap()
		if err != nil {
			return nil, err
		}
		out["cache_creation"] = cacheCreation
	}
	setMapString(out, "inference_geo", c.InferenceGeo)
	setMapString(out, "service_tier", c.ServiceTier)
	if c.ServerToolUse != nil {
		serverToolUse, err := c.ServerToolUse.ToMap()
		if err != nil {
			return nil, err
		}
		out["server_tool_use"] = serverToolUse
	}
	if len(c.Iterations) > 0 {
		iterations, err := claudeUsageByModelListToMaps(c.Iterations)
		if err != nil {
			return nil, err
		}
		out["iterations"] = iterations
	}
	return out, nil
}

type ClaudeContainer struct {
	Id        string `json:"id"`
	ExpiresAt string `json:"expires_at"`
}

func (c *ClaudeContainer) FromMap(m map[string]any) error {
	var err error
	c.Id, err = stringValue(m, "id")
	if err != nil {
		return err
	}
	c.ExpiresAt, err = stringValue(m, "expires_at")
	return err
}

func (c *ClaudeContainer) ToMap() (map[string]any, error) {
	return map[string]any{
		"id":         c.Id,
		"expires_at": c.ExpiresAt,
	}, nil
}

type ClaudeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (c *ClaudeError) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	c.Message, err = stringValue(m, "message")
	return err
}

func (c *ClaudeError) ToMap() (map[string]any, error) {
	return map[string]any{
		"type":    c.Type,
		"message": c.Message,
	}, nil
}

type ClaudeStopDetails struct {
	Type                    string  `json:"type,omitempty"`
	Category                string  `json:"category,omitempty"`
	Explanation             string  `json:"explanation,omitempty"`
	FallbackCreditToken     *string `json:"fallback_credit_token,omitempty"`
	FallbackHasPrefillClaim *bool   `json:"fallback_has_prefill_claim,omitempty"`
}

func (s *ClaudeStopDetails) FromMap(m map[string]any) error {
	var err error
	s.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	s.Category, err = stringValue(m, "category")
	if err != nil {
		return err
	}
	s.Explanation, err = stringValue(m, "explanation")
	if err != nil {
		return err
	}
	s.FallbackCreditToken, err = stringPtrValue(m, "fallback_credit_token")
	if err != nil {
		return err
	}
	s.FallbackHasPrefillClaim, err = boolPtrValue(m, "fallback_has_prefill_claim")
	return err
}

func (s *ClaudeStopDetails) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", s.Type)
	setMapString(out, "category", s.Category)
	setMapString(out, "explanation", s.Explanation)
	if s.FallbackCreditToken != nil {
		out["fallback_credit_token"] = *s.FallbackCreditToken
	}
	if s.FallbackHasPrefillClaim != nil {
		out["fallback_has_prefill_claim"] = *s.FallbackHasPrefillClaim
	}
	return out, nil
}

func decodeClaudeStopDetailsPtrFromMapField(m map[string]any, key string) (*ClaudeStopDetails, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("field %q: expected object, got %T", key, v)
	}
	var s ClaudeStopDetails
	if err := s.FromMap(obj); err != nil {
		return nil, err
	}
	return &s, nil
}

type ClaudeResponse struct {
	Id           string             `json:"id"`
	Container    *ClaudeContainer   `json:"container"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []ClaudeContent    `json:"content"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason"`
	StopSequence string             `json:"stop_sequence"`
	StopDetails  *ClaudeStopDetails `json:"stop_details,omitempty"`
	Usage        *ClaudeUsage       `json:"usage"`
	Error        *ClaudeError       `json:"error,omitempty"`
}

func (c *ClaudeResponse) UnmarshalJSON(b []byte) error {
	type wire struct {
		Id           string             `json:"id"`
		Container    *ClaudeContainer   `json:"container"`
		Type         string             `json:"type"`
		Role         string             `json:"role"`
		Content      []json.RawMessage  `json:"content"`
		Model        string             `json:"model"`
		StopReason   string             `json:"stop_reason"`
		StopSequence string             `json:"stop_sequence"`
		StopDetails  *ClaudeStopDetails `json:"stop_details,omitempty"`
		Usage        *ClaudeUsage       `json:"usage"`
		Error        *ClaudeError       `json:"error,omitempty"`
	}
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*c = ClaudeResponse{
		Id:           w.Id,
		Container:    w.Container,
		Type:         w.Type,
		Role:         w.Role,
		Model:        w.Model,
		StopReason:   w.StopReason,
		StopSequence: w.StopSequence,
		StopDetails:  w.StopDetails,
		Usage:        w.Usage,
		Error:        w.Error,
	}
	items, err := decodeClaudeContentList(w.Content)
	if err != nil {
		return err
	}
	c.Content = items
	return nil
}

func (c *ClaudeResponse) FromMap(m map[string]any) error {
	var err error
	c.Id, err = stringValue(m, "id")
	if err != nil {
		return err
	}
	c.Container, err = decodeClaudeContainerPtrFromMapField(m, "container")
	if err != nil {
		return err
	}
	c.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	c.Role, err = stringValue(m, "role")
	if err != nil {
		return err
	}
	c.Content, err = decodeClaudeContentListMapField(m, "content")
	if err != nil {
		return err
	}
	c.Model, err = stringValue(m, "model")
	if err != nil {
		return err
	}
	c.StopReason, err = stringValue(m, "stop_reason")
	if err != nil {
		return err
	}
	c.StopSequence, err = stringValue(m, "stop_sequence")
	if err != nil {
		return err
	}
	c.StopDetails, err = decodeClaudeStopDetailsPtrFromMapField(m, "stop_details")
	if err != nil {
		return err
	}
	c.Usage, err = decodeClaudeUsagePtrFromMapField(m, "usage")
	if err != nil {
		return err
	}
	c.Error, err = decodeClaudeErrorPtrFromMapField(m, "error")
	return err
}

func (c *ClaudeResponse) ToMap() (map[string]any, error) {
	out := map[string]any{
		"id":            c.Id,
		"type":          c.Type,
		"role":          c.Role,
		"model":         c.Model,
		"stop_reason":   c.StopReason,
		"stop_sequence": c.StopSequence,
	}
	if c.Container != nil {
		container, err := c.Container.ToMap()
		if err != nil {
			return nil, err
		}
		out["container"] = container
	}
	content, err := claudeContentListToMaps(c.Content)
	if err != nil {
		return nil, err
	}
	out["content"] = content
	if c.StopDetails != nil {
		stopDetails, err := c.StopDetails.ToMap()
		if err != nil {
			return nil, err
		}
		out["stop_details"] = stopDetails
	}
	if c.Usage != nil {
		usage, err := c.Usage.ToMap()
		if err != nil {
			return nil, err
		}
		out["usage"] = usage
	}
	if c.Error != nil {
		errorMap, err := c.Error.ToMap()
		if err != nil {
			return nil, err
		}
		out["error"] = errorMap
	}
	return out, nil
}

type ClaudeMessageRequest = ClaudeRequest
type ClaudeMessageResponse = ClaudeResponse

func decodeCacheControlPtrFromMapField(m map[string]any) (*CacheControl, error) {
	const key = "cache_control"
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out CacheControl
	return &out, out.FromMap(mv)
}

func decodeClaudeMetadataPtrFromMapField(m map[string]any, key string) (*ClaudeMetadata, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ClaudeMetadata
	return &out, out.FromMap(mv)
}

func decodeClaudeSourceMapField(m map[string]any, key string) (ClaudeSource, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	return decodeClaudeSourceFromMap(mv)
}

func decodeClaudeCitationListFromMapField(m map[string]any, key string) ([]ClaudeCitation, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	out := make([]ClaudeCitation, 0, len(items))
	for _, item := range items {
		citation, err := decodeClaudeCitationFromMap(item)
		if err != nil {
			return nil, err
		}
		out = append(out, citation)
	}
	return out, nil
}

func decodeClaudeCitationsConfigPtrFromMapField(m map[string]any, key string) (*ClaudeCitationsConfig, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ClaudeCitationsConfig
	return &out, out.FromMap(mv)
}

func decodeClaudeCallerPtrFromMapField(m map[string]any, key string) (*ClaudeCaller, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ClaudeCaller
	return &out, out.FromMap(mv)
}

func decodeClaudeContentMapField(m map[string]any) (ClaudeContent, error) {
	const key = "content"
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	return decodeClaudeContentFromMap(mv)
}

func decodeClaudeContentListMapField(m map[string]any, key string) ([]ClaudeContent, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	return decodeClaudeContentListFromAny(v)
}

func decodeClaudeCodeExecutionOutputListFromMapField(m map[string]any, key string) ([]ClaudeCodeExecutionOutputContent, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]ClaudeCodeExecutionOutputContent, 0, len(items))
	for _, item := range items {
		var v ClaudeCodeExecutionOutputContent
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeClaudeBashCodeExecutionOutputListFromMapField(m map[string]any, key string) ([]ClaudeBashCodeExecutionOutputContent, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]ClaudeBashCodeExecutionOutputContent, 0, len(items))
	for _, item := range items {
		var v ClaudeBashCodeExecutionOutputContent
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeClaudeToolReferenceListFromMapField(m map[string]any, key string) ([]ClaudeToolReferenceContent, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]ClaudeToolReferenceContent, 0, len(items))
	for _, item := range items {
		var v ClaudeToolReferenceContent
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeClaudeInputSchemaPtrFromMapField(m map[string]any, key string) (*ClaudeInputSchema, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ClaudeInputSchema
	return &out, out.FromMap(mv)
}

func decodeClaudeUserLocationPtrFromMapField(m map[string]any, key string) (*ClaudeUserLocation, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ClaudeUserLocation
	return &out, out.FromMap(mv)
}

func decodeClaudeJsonOutputFormatPtrFromMapField(m map[string]any, key string) (*ClaudeJsonOutputFormat, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ClaudeJsonOutputFormat
	return &out, out.FromMap(mv)
}

func decodeClaudeOutputConfigPtrFromMapField(m map[string]any, key string) (*ClaudeOutputConfig, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ClaudeOutputConfig
	return &out, out.FromMap(mv)
}

func decodeThinkingConfigPtrFromMapField(m map[string]any, key string) (*ThinkingConfig, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ThinkingConfig
	return &out, out.FromMap(mv)
}

func decodeClaudeToolChoicePtrFromMapField(m map[string]any, key string) (*ClaudeToolChoice, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ClaudeToolChoice
	return &out, out.FromMap(mv)
}

func decodeFallbackListFromMapField(m map[string]any, key string) ([]*Fallback, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array, got %T", key, v)
	}
	out := make([]*Fallback, 0, len(items))
	for i, item := range items {
		if item == nil {
			continue
		}
		mv, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d]: expected object, got %T", key, i, item)
		}
		var fb Fallback
		if err := fb.FromMap(mv); err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", key, i, err)
		}
		if fb.Model == "" {
			return nil, fmt.Errorf("%s[%d]: model is required", key, i)
		}
		out = append(out, &fb)
	}
	return out, nil
}

func fallbackListToMaps(items []*Fallback) ([]any, error) {
	out := make([]any, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		m, err := item.ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func decodeClaudeToolListFromMapField(m map[string]any, key string) ([]ClaudeTool, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]ClaudeTool, 0, len(items))
	for _, item := range items {
		var v ClaudeTool
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeClaudeMessageListFromMapField(m map[string]any, key string) ([]ClaudeMessage, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]ClaudeMessage, 0, len(items))
	for _, item := range items {
		var v ClaudeMessage
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeCacheCreationUsageDetailPtrFromMapField(m map[string]any, key string) (*CacheCreationUsageDetail, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out CacheCreationUsageDetail
	return &out, out.FromMap(mv)
}

func decodeClaudeServerToolUsagePtrFromMapField(m map[string]any, key string) (*ClaudeServerToolUsage, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ClaudeServerToolUsage
	return &out, out.FromMap(mv)
}

func decodeClaudeContainerPtrFromMapField(m map[string]any, key string) (*ClaudeContainer, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ClaudeContainer
	return &out, out.FromMap(mv)
}

func decodeClaudeUsageByModelListFromMapField(m map[string]any, key string) ([]ClaudeUsageByModel, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]ClaudeUsageByModel, 0, len(items))
	for _, item := range items {
		var v ClaudeUsageByModel
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func claudeUsageByModelListToMaps(items []ClaudeUsageByModel) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		m, err := items[i].ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func decodeClaudeUsagePtrFromMapField(m map[string]any, key string) (*ClaudeUsage, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ClaudeUsage
	return &out, out.FromMap(mv)
}

func decodeClaudeErrorPtrFromMapField(m map[string]any, key string) (*ClaudeError, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ClaudeError
	return &out, out.FromMap(mv)
}

func claudeSourceToMap(v ClaudeSource) (map[string]any, error) {
	if v == nil {
		return nil, nil
	}
	switch typed := v.(type) {
	case interface {
		ToMap() (map[string]any, error)
	}:
		return typed.ToMap()
	default:
		return nil, fmt.Errorf("unsupported claude source type %T", v)
	}
}

func claudeCitationListToMaps(items []ClaudeCitation) ([]any, error) {
	out := make([]any, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case interface {
			ToMap() (map[string]any, error)
		}:
			mv, err := typed.ToMap()
			if err != nil {
				return nil, err
			}
			out = append(out, mv)
		default:
			return nil, fmt.Errorf("unsupported claude citation type %T", item)
		}
	}
	return out, nil
}

func claudeContentToMap(v ClaudeContent) (map[string]any, error) {
	if v == nil {
		return nil, nil
	}
	switch typed := v.(type) {
	case interface {
		ToMap() (map[string]any, error)
	}:
		return typed.ToMap()
	default:
		return nil, fmt.Errorf("unsupported claude content type %T", v)
	}
}

func claudeContentListToMaps(items []ClaudeContent) ([]any, error) {
	out := make([]any, 0, len(items))
	for _, item := range items {
		mv, err := claudeContentToMap(item)
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func claudeCodeExecutionOutputListToMaps(items []ClaudeCodeExecutionOutputContent) ([]any, error) {
	out := make([]any, 0, len(items))
	for _, item := range items {
		mv, err := item.ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func claudeBashCodeExecutionOutputListToMaps(items []ClaudeBashCodeExecutionOutputContent) ([]any, error) {
	out := make([]any, 0, len(items))
	for _, item := range items {
		mv, err := item.ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func claudeToolReferenceListToMaps(items []ClaudeToolReferenceContent) ([]any, error) {
	out := make([]any, 0, len(items))
	for _, item := range items {
		mv, err := item.ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func claudeMessageContentToAny(v any) (any, error) {
	switch typed := v.(type) {
	case nil:
		return nil, nil
	case string:
		return typed, nil
	case []ClaudeContent:
		return claudeContentListToMaps(typed)
	default:
		return nil, fmt.Errorf("unsupported claude message content type %T", v)
	}
}

func toolResultContentToAny(v any) (any, error) {
	switch typed := v.(type) {
	case nil:
		return nil, nil
	case string:
		return typed, nil
	case []ClaudeContent:
		return claudeContentListToMaps(typed)
	default:
		return nil, fmt.Errorf("unsupported tool result content type %T", v)
	}
}

func webSearchToolResultToAny(v any) (any, error) {
	switch typed := v.(type) {
	case nil:
		return nil, nil
	case []ClaudeWebSearchResultContent:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			mv, err := item.ToMap()
			if err != nil {
				return nil, err
			}
			out = append(out, mv)
		}
		return out, nil
	case *ClaudeWebSearchToolRequestErrorContent:
		return typed.ToMap()
	default:
		return nil, fmt.Errorf("unsupported web search tool result content type %T", v)
	}
}

func claudeMessageListToMaps(items []ClaudeMessage) ([]any, error) {
	out := make([]any, 0, len(items))
	for _, item := range items {
		mv, err := item.ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func claudeToolListToMaps(items []ClaudeTool) ([]any, error) {
	out := make([]any, 0, len(items))
	for _, item := range items {
		mv, err := item.ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func claudeSystemToAny(v any) (any, error) {
	switch typed := v.(type) {
	case nil:
		return nil, nil
	case string:
		return typed, nil
	case []ClaudeTextContent:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			mv, err := item.ToMap()
			if err != nil {
				return nil, err
			}
			out = append(out, mv)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported claude system type %T", v)
	}
}

func decodeClaudeSourceFromMap(m map[string]any) (ClaudeSource, error) {
	typeName, ok := m["type"].(string)
	if !ok {
		return nil, fmt.Errorf("claude source type must be string")
	}
	switch typeName {
	case "base64":
		var v ClaudeBase64Source
		return &v, v.FromMap(m)
	case "text":
		var v ClaudeTextSource
		return &v, v.FromMap(m)
	case "url":
		var v ClaudeURLSource
		return &v, v.FromMap(m)
	case "content":
		var v ClaudeContentSource
		return &v, v.FromMap(m)
	default:
		raw, _ := json.Marshal(m)
		return &ClaudeUnknownSource{ClaudeBaseSource: ClaudeBaseSource{Type: typeName}, Raw: raw}, nil
	}
}

func decodeClaudeCitationFromMap(m map[string]any) (ClaudeCitation, error) {
	typeName, ok := m["type"].(string)
	if !ok {
		return nil, fmt.Errorf("claude citation type must be string")
	}
	switch typeName {
	case "char_location":
		var v ClaudeCitationCharLocation
		return &v, v.FromMap(m)
	case "page_location":
		var v ClaudeCitationPageLocation
		return &v, v.FromMap(m)
	case "content_block_location":
		var v ClaudeCitationContentBlockLocation
		return &v, v.FromMap(m)
	case "web_search_result_location":
		var v ClaudeCitationWebSearchResultLocation
		return &v, v.FromMap(m)
	case "search_result_location":
		var v ClaudeCitationSearchResultLocation
		return &v, v.FromMap(m)
	default:
		raw, _ := json.Marshal(m)
		return &ClaudeUnknownCitation{ClaudeBaseCitation: ClaudeBaseCitation{Type: typeName}, Raw: raw}, nil
	}
}

func decodeClaudeContentFromMap(m map[string]any) (ClaudeContent, error) {
	typeName, ok := m["type"].(string)
	if !ok {
		return nil, fmt.Errorf("claude content type must be string")
	}
	if content, matched, err := decodeClaudeContentFromMapCore(typeName, m); matched {
		return content, err
	}
	if content, matched, err := decodeClaudeContentFromMapExtended(typeName, m); matched {
		return content, err
	}
	raw, _ := json.Marshal(m)
	return &ClaudeUnknownContent{ClaudeBaseContent: ClaudeBaseContent{Type: typeName}, Raw: raw}, nil
}

func decodeClaudeContentFromMapCore(typeName string, m map[string]any) (ClaudeContent, bool, error) {
	switch typeName {
	case "text":
		var v ClaudeTextContent
		return &v, true, v.FromMap(m)
	case "image":
		var v ClaudeImageContent
		return &v, true, v.FromMap(m)
	case "document":
		var v ClaudeDocumentContent
		return &v, true, v.FromMap(m)
	case "search_result":
		var v ClaudeSearchResultContent
		return &v, true, v.FromMap(m)
	case "thinking":
		var v ClaudeThinkingContent
		return &v, true, v.FromMap(m)
	case "redacted_thinking":
		var v ClaudeRedactedThinkingContent
		return &v, true, v.FromMap(m)
	case "tool_use":
		var v ClaudeToolUseContent
		return &v, true, v.FromMap(m)
	case "tool_result":
		var v ClaudeToolResultContent
		return &v, true, v.FromMap(m)
	case "tool_reference":
		var v ClaudeToolReferenceContent
		return &v, true, v.FromMap(m)
	case "server_tool_use":
		var v ClaudeServerToolUseContent
		return &v, true, v.FromMap(m)
	case "web_search_result":
		var v ClaudeWebSearchResultContent
		return &v, true, v.FromMap(m)
	case "web_search_tool_result_error":
		var v ClaudeWebSearchToolRequestErrorContent
		return &v, true, v.FromMap(m)
	case "web_search_tool_result":
		var v ClaudeWebSearchToolResultContent
		return &v, true, v.FromMap(m)
	case "web_fetch_tool_result_error":
		var v ClaudeWebFetchToolResultErrorContent
		return &v, true, v.FromMap(m)
	case "web_fetch_result":
		var v ClaudeWebFetchResultContent
		return &v, true, v.FromMap(m)
	case "web_fetch_tool_result":
		var v ClaudeWebFetchToolResultContent
		return &v, true, v.FromMap(m)
	case "code_execution_tool_result_error":
		var v ClaudeCodeExecutionToolResultErrorContent
		return &v, true, v.FromMap(m)
	case "code_execution_output":
		var v ClaudeCodeExecutionOutputContent
		return &v, true, v.FromMap(m)
	case "code_execution_result":
		var v ClaudeCodeExecutionResultContent
		return &v, true, v.FromMap(m)
	case "encrypted_code_execution_result":
		var v ClaudeEncryptedCodeExecutionResultContent
		return &v, true, v.FromMap(m)
	case "code_execution_tool_result":
		var v ClaudeCodeExecutionToolResultContent
		return &v, true, v.FromMap(m)
	default:
		return nil, false, nil
	}
}

func decodeClaudeContentFromMapExtended(typeName string, m map[string]any) (ClaudeContent, bool, error) {
	switch typeName {
	case "bash_code_execution_tool_result_error":
		var v ClaudeBashCodeExecutionToolResultErrorContent
		return &v, true, v.FromMap(m)
	case "bash_code_execution_output":
		var v ClaudeBashCodeExecutionOutputContent
		return &v, true, v.FromMap(m)
	case "bash_code_execution_result":
		var v ClaudeBashCodeExecutionResultContent
		return &v, true, v.FromMap(m)
	case "bash_code_execution_tool_result":
		var v ClaudeBashCodeExecutionToolResultContent
		return &v, true, v.FromMap(m)
	case "text_editor_code_execution_tool_result_error":
		var v ClaudeTextEditorCodeExecutionResultErrorContent
		return &v, true, v.FromMap(m)
	case "text_editor_code_execution_view_result":
		var v ClaudeTextEditorCodeExecutionViewResultContent
		return &v, true, v.FromMap(m)
	case "text_editor_code_execution_create_result":
		var v ClaudeTextEditorCodeExecutionCreateResultContent
		return &v, true, v.FromMap(m)
	case "text_editor_code_execution_str_replace_result":
		var v ClaudeTextEditorCodeExecutionStrReplaceResultContent
		return &v, true, v.FromMap(m)
	case "text_editor_code_execution_tool_result":
		var v ClaudeTextEditorCodeExecutionToolResultContent
		return &v, true, v.FromMap(m)
	case "tool_search_tool_result_error":
		var v ClaudeToolSearchToolResultErrorContent
		return &v, true, v.FromMap(m)
	case "tool_search_tool_search_result":
		var v ClaudeToolSearchToolSearchResultContent
		return &v, true, v.FromMap(m)
	case "tool_search_tool_result":
		var v ClaudeToolSearchToolResultContent
		return &v, true, v.FromMap(m)
	case "container_upload":
		var v ClaudeContainerUploadContent
		return &v, true, v.FromMap(m)
	default:
		return nil, false, nil
	}
}

func decodeClaudeThinkingFromMap(m map[string]any) (ThinkingConfigInterface, error) {
	typeName, ok := m["type"].(string)
	if !ok {
		return nil, fmt.Errorf("thinking config type must be string")
	}
	switch typeName {
	case "enabled":
		var v ThinkingConfigEnabled
		return &v, v.FromMap(m)
	case "adaptive":
		var v ThinkingConfigAdaptive
		return &v, v.FromMap(m)
	case "disabled":
		var v ThinkingConfigDisabled
		return &v, v.FromMap(m)
	default:
		raw, _ := json.Marshal(m)
		return &ThinkingConfigUnknown{BaseThinkingConfig: BaseThinkingConfig{Type: typeName}, Raw: raw}, nil
	}
}

func decodeClaudeContentListFromAny(v any) ([]ClaudeContent, error) {
	if v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("expected content list, got %T", v)
	}
	result := make([]ClaudeContent, 0, len(items))
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected content item map, got %T", item)
		}
		decoded, err := decodeClaudeContentFromMap(itemMap)
		if err != nil {
			return nil, err
		}
		result = append(result, decoded)
	}
	return result, nil
}

func decodeClaudeMessageContentFromMap(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	return decodeClaudeContentListFromAny(v)
}

func decodeToolResultContentFromMap(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	return decodeClaudeContentListFromAny(v)
}

func decodeWebSearchToolResultFromMap(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if items, ok := v.([]any); ok {
		results := make([]ClaudeWebSearchResultContent, 0, len(items))
		for _, item := range items {
			itemMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected web search result map, got %T", item)
			}
			var result ClaudeWebSearchResultContent
			if err := result.FromMap(itemMap); err != nil {
				return nil, err
			}
			results = append(results, result)
		}
		return results, nil
	}
	itemMap, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected web search tool result content, got %T", v)
	}
	var errContent ClaudeWebSearchToolRequestErrorContent
	if err := errContent.FromMap(itemMap); err != nil {
		return nil, err
	}
	return &errContent, nil
}

func decodeClaudeSystemFromMap(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("expected system string or block list, got %T", v)
	}
	blocks := make([]ClaudeTextContent, 0, len(items))
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected system block map, got %T", item)
		}
		content, err := decodeClaudeContentFromMap(itemMap)
		if err != nil {
			return nil, err
		}
		textBlock, ok := content.(*ClaudeTextContent)
		if !ok {
			return nil, fmt.Errorf("expected system text block, got %T", content)
		}
		blocks = append(blocks, *textBlock)
	}
	return blocks, nil
}

func decodeClaudeSource(raw json.RawMessage) (ClaudeSource, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var base ClaudeBaseSource
	if err := json.Unmarshal(trimmed, &base); err != nil {
		return nil, err
	}
	switch base.Type {
	case "base64":
		var v ClaudeBase64Source
		return &v, json.Unmarshal(trimmed, &v)
	case "text":
		var v ClaudeTextSource
		return &v, json.Unmarshal(trimmed, &v)
	case "url":
		var v ClaudeURLSource
		return &v, json.Unmarshal(trimmed, &v)
	case "content":
		var v ClaudeContentSource
		return &v, json.Unmarshal(trimmed, &v)
	default:
		return nil, fmt.Errorf("unsupported claude source type: %q", base.Type)
	}
}

func decodeClaudeCitation(raw json.RawMessage) (ClaudeCitation, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var base ClaudeBaseCitation
	if err := json.Unmarshal(trimmed, &base); err != nil {
		return nil, err
	}
	switch base.Type {
	case "char_location":
		var v ClaudeCitationCharLocation
		return &v, json.Unmarshal(trimmed, &v)
	case "page_location":
		var v ClaudeCitationPageLocation
		return &v, json.Unmarshal(trimmed, &v)
	case "content_block_location":
		var v ClaudeCitationContentBlockLocation
		return &v, json.Unmarshal(trimmed, &v)
	case "web_search_result_location":
		var v ClaudeCitationWebSearchResultLocation
		return &v, json.Unmarshal(trimmed, &v)
	case "search_result_location":
		var v ClaudeCitationSearchResultLocation
		return &v, json.Unmarshal(trimmed, &v)
	default:
		return nil, fmt.Errorf("unsupported claude citation type: %q", base.Type)
	}
}

func decodeClaudeContent(raw json.RawMessage) (ClaudeContent, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var base ClaudeBaseContent
	if err := json.Unmarshal(trimmed, &base); err != nil {
		return nil, err
	}
	if content, matched, err := decodeClaudeContentCore(base.Type, trimmed); matched {
		return content, err
	}
	if content, matched, err := decodeClaudeContentExtended(base.Type, trimmed); matched {
		return content, err
	}
	return nil, fmt.Errorf("unsupported claude content type: %q", base.Type)
}

func decodeClaudeContentCore(typeName string, raw []byte) (ClaudeContent, bool, error) {
	switch typeName {
	case "text":
		var v ClaudeTextContent
		return &v, true, json.Unmarshal(raw, &v)
	case "image":
		var v ClaudeImageContent
		return &v, true, json.Unmarshal(raw, &v)
	case "document":
		var v ClaudeDocumentContent
		return &v, true, json.Unmarshal(raw, &v)
	case "search_result":
		var v ClaudeSearchResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "thinking":
		var v ClaudeThinkingContent
		return &v, true, json.Unmarshal(raw, &v)
	case "redacted_thinking":
		var v ClaudeRedactedThinkingContent
		return &v, true, json.Unmarshal(raw, &v)
	case "tool_use":
		var v ClaudeToolUseContent
		return &v, true, json.Unmarshal(raw, &v)
	case "tool_result":
		var v ClaudeToolResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "tool_reference":
		var v ClaudeToolReferenceContent
		return &v, true, json.Unmarshal(raw, &v)
	case "server_tool_use":
		var v ClaudeServerToolUseContent
		return &v, true, json.Unmarshal(raw, &v)
	case "web_search_result":
		var v ClaudeWebSearchResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "web_search_tool_result_error":
		var v ClaudeWebSearchToolRequestErrorContent
		return &v, true, json.Unmarshal(raw, &v)
	case "web_search_tool_result":
		var v ClaudeWebSearchToolResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "web_fetch_tool_result_error":
		var v ClaudeWebFetchToolResultErrorContent
		return &v, true, json.Unmarshal(raw, &v)
	case "web_fetch_result":
		var v ClaudeWebFetchResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "web_fetch_tool_result":
		var v ClaudeWebFetchToolResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "code_execution_tool_result_error":
		var v ClaudeCodeExecutionToolResultErrorContent
		return &v, true, json.Unmarshal(raw, &v)
	case "code_execution_output":
		var v ClaudeCodeExecutionOutputContent
		return &v, true, json.Unmarshal(raw, &v)
	case "code_execution_result":
		var v ClaudeCodeExecutionResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "encrypted_code_execution_result":
		var v ClaudeEncryptedCodeExecutionResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "code_execution_tool_result":
		var v ClaudeCodeExecutionToolResultContent
		return &v, true, json.Unmarshal(raw, &v)
	default:
		return nil, false, nil
	}
}

func decodeClaudeContentExtended(typeName string, raw []byte) (ClaudeContent, bool, error) {
	switch typeName {
	case "bash_code_execution_tool_result_error":
		var v ClaudeBashCodeExecutionToolResultErrorContent
		return &v, true, json.Unmarshal(raw, &v)
	case "bash_code_execution_output":
		var v ClaudeBashCodeExecutionOutputContent
		return &v, true, json.Unmarshal(raw, &v)
	case "bash_code_execution_result":
		var v ClaudeBashCodeExecutionResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "bash_code_execution_tool_result":
		var v ClaudeBashCodeExecutionToolResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "text_editor_code_execution_tool_result_error":
		var v ClaudeTextEditorCodeExecutionResultErrorContent
		return &v, true, json.Unmarshal(raw, &v)
	case "text_editor_code_execution_view_result":
		var v ClaudeTextEditorCodeExecutionViewResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "text_editor_code_execution_create_result":
		var v ClaudeTextEditorCodeExecutionCreateResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "text_editor_code_execution_str_replace_result":
		var v ClaudeTextEditorCodeExecutionStrReplaceResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "text_editor_code_execution_tool_result":
		var v ClaudeTextEditorCodeExecutionToolResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "tool_search_tool_result_error":
		var v ClaudeToolSearchToolResultErrorContent
		return &v, true, json.Unmarshal(raw, &v)
	case "tool_search_tool_search_result":
		var v ClaudeToolSearchToolSearchResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "tool_search_tool_result":
		var v ClaudeToolSearchToolResultContent
		return &v, true, json.Unmarshal(raw, &v)
	case "container_upload":
		var v ClaudeContainerUploadContent
		return &v, true, json.Unmarshal(raw, &v)
	default:
		return nil, false, nil
	}
}

func decodeClaudeContentList(raws []json.RawMessage) ([]ClaudeContent, error) {
	if len(raws) == 0 {
		return nil, nil
	}
	items := make([]ClaudeContent, 0, len(raws))
	for _, raw := range raws {
		item, err := decodeClaudeContent(raw)
		if err != nil {
			return nil, err
		}
		if item != nil {
			items = append(items, item)
		}
	}
	return items, nil
}

func decodeClaudeThinking(raw json.RawMessage) (ThinkingConfigInterface, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var base BaseThinkingConfig
	if err := json.Unmarshal(trimmed, &base); err != nil {
		return nil, err
	}
	switch base.Type {
	case "enabled":
		var v ThinkingConfigEnabled
		return &v, json.Unmarshal(trimmed, &v)
	case "adaptive":
		var v ThinkingConfigAdaptive
		return &v, json.Unmarshal(trimmed, &v)
	case "disabled":
		var v ThinkingConfigDisabled
		return &v, json.Unmarshal(trimmed, &v)
	default:
		return nil, fmt.Errorf("unsupported thinking config type: %q", base.Type)
	}
}
