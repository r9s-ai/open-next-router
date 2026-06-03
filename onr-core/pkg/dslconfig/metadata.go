package dslconfig

import (
	"fmt"
	"strings"
)

// ProviderMetadata describes provider identity and capacity signal profile.
// Empty DSL fields are normalized at provider-file build time:
// provider_family defaults to the provider name, and signal_profile defaults
// to provider_family.
type ProviderMetadata struct {
	ProviderFamily string `json:"providerFamily,omitempty"`
	SignalProfile  string `json:"signalProfile,omitempty"`
}

func normalizeProviderMetadata(providerName string, metadata ProviderMetadata) (ProviderMetadata, error) {
	providerName = normalizeProviderName(providerName)
	providerFamily := normalizeProviderName(metadata.ProviderFamily)
	if providerFamily == "" {
		providerFamily = providerName
	}
	signalProfile := normalizeProviderName(metadata.SignalProfile)
	if signalProfile == "" {
		signalProfile = providerFamily
	}
	out := ProviderMetadata{
		ProviderFamily: providerFamily,
		SignalProfile:  signalProfile,
	}
	if err := validateMetadataToken("provider_family", out.ProviderFamily); err != nil {
		return ProviderMetadata{}, err
	}
	if err := validateMetadataToken("signal_profile", out.SignalProfile); err != nil {
		return ProviderMetadata{}, err
	}
	return out, nil
}

func validateMetadataToken(field string, value string) error {
	if !providerNamePattern.MatchString(value) {
		return fmt.Errorf("invalid metadata %s %q, expected pattern %s", field, value, providerNamePattern.String())
	}
	return nil
}

func parseProviderMetadataFromContent(path string, content string, providerName string) (ProviderMetadata, error) {
	s := newScanner(path, content)
	want := normalizeProviderName(providerName)
	for {
		tok := s.nextNonTrivia()
		if tok.kind == tokEOF {
			return normalizeProviderMetadata(want, ProviderMetadata{})
		}
		if tok.kind != tokIdent || tok.text != providerKeyword {
			continue
		}
		nameTok := s.nextNonTrivia()
		if nameTok.kind != tokString {
			return ProviderMetadata{}, s.errAt(nameTok, "expected provider name string literal")
		}
		lb := s.nextNonTrivia()
		if lb.kind != tokLBrace {
			return ProviderMetadata{}, s.errAt(lb, "expected '{' after provider name")
		}
		name := normalizeProviderName(unquoteString(nameTok.text))
		if name != want {
			if err := skipBalancedBraces(s); err != nil {
				return ProviderMetadata{}, err
			}
			continue
		}
		return parseProviderMetadataBody(s, want)
	}
}

func parseProviderMetadataBody(s *scanner, providerName string) (ProviderMetadata, error) {
	var metadata ProviderMetadata
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return ProviderMetadata{}, s.errAt(tok, "unexpected EOF in provider block")
		case tokRBrace:
			return normalizeProviderMetadata(providerName, metadata)
		case tokIdent:
			if tok.text == "metadata" {
				if err := parseMetadataBlock(s, &metadata); err != nil {
					return ProviderMetadata{}, err
				}
				continue
			}
			if err := skipStmtOrBlock(s); err != nil {
				return ProviderMetadata{}, err
			}
		default:
			// Ignore trivia-like punctuation that the main parser already tolerates.
		}
	}
}

func parseMetadataBlock(s *scanner, metadata *ProviderMetadata) error {
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return s.errAt(lb, "expected '{' after metadata")
	}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in metadata block")
		case tokRBrace:
			return nil
		case tokIdent:
			value, err := parseMetadataValueStmt(s, tok.text)
			if err != nil {
				return err
			}
			switch tok.text {
			case "provider_family":
				metadata.ProviderFamily = value
			case "signal_profile":
				metadata.SignalProfile = value
			default:
				return s.errAt(tok, "unknown metadata directive "+tok.text)
			}
		default:
			return s.errAt(tok, "metadata directive expected")
		}
	}
}

func parseMetadataValueStmt(s *scanner, field string) (string, error) {
	tok := s.nextNonTrivia()
	switch tok.kind {
	case tokIdent:
		// ok
	case tokString:
		// quoted strings are accepted for compatibility, but new configs should use tokens.
	default:
		return "", s.errAt(tok, field+" expects identifier token")
	}
	if err := consumeSemicolon(s, field); err != nil {
		return "", err
	}
	value := strings.TrimSpace(tok.text)
	if tok.kind == tokString {
		value = strings.TrimSpace(unquoteString(tok.text))
	}
	value = normalizeProviderName(value)
	if value == "" {
		return "", s.errAt(tok, field+" must be non-empty")
	}
	if err := validateMetadataToken(field, value); err != nil {
		return "", s.errAt(tok, err.Error())
	}
	return value, nil
}
