package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-admin/internal/store"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/keystore"
	"github.com/r9s-ai/open-next-router/pkg/config"
	"github.com/r9s-ai/open-next-router/pkg/controlplane"
	"github.com/spf13/cobra"
)

func newAccessKeyCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "access-key", Short: "Manage Redis-backed client access keys"}
	cmd.AddCommand(newAccessKeyCreateCmd(), newAccessKeyListCmd(), newAccessKeyRevokeCmd(), newAccessKeyRotateCmd(), newAccessKeyMigrateCmd())
	return cmd
}

type accessKeyOptions struct {
	cfgPath     string
	name        string
	subjectType string
	subjectID   string
}

func newAccessKeyCreateCmd() *cobra.Command {
	opts := accessKeyOptions{cfgPath: "onr.yaml", subjectType: "api_key"}
	cmd := &cobra.Command{Use: "create", Short: "Create a Redis-backed client access key", RunE: func(cmd *cobra.Command, args []string) error {
		client, err := openControlPlane(opts.cfgPath)
		if err != nil {
			return err
		}
		defer func() { _ = client.Close() }()
		secret, err := controlplane.NewAccessKeySecret()
		if err != nil {
			return err
		}
		if strings.TrimSpace(opts.name) == "" {
			return errors.New("--name is required")
		}
		record := controlplane.AccessKeyRecord{Name: opts.name, SecretHash: client.HashAccessKey(secret), Status: "active", SubjectType: opts.subjectType, SubjectID: opts.subjectID}
		if err := client.CreateAccessKey(context.Background(), record); err != nil {
			return err
		}
		fmt.Printf("name=%s subject=%s/%s secret=%s\n", record.Name, record.SubjectType, record.SubjectID, secret)
		return nil
	}}
	addAccessKeyFlags(cmd, &opts, true)
	return cmd
}

func newAccessKeyListCmd() *cobra.Command {
	opts := accessKeyOptions{cfgPath: "onr.yaml"}
	cmd := &cobra.Command{Use: "list", Short: "List Redis-backed client access keys", RunE: func(cmd *cobra.Command, args []string) error {
		client, err := openControlPlane(opts.cfgPath)
		if err != nil {
			return err
		}
		defer func() { _ = client.Close() }()
		records, err := client.ListAccessKeys(context.Background())
		if err != nil {
			return err
		}
		sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })
		for _, record := range records {
			fmt.Printf("name=%s status=%s subject=%s/%s version=%d\n", record.Name, record.Status, record.SubjectType, record.SubjectID, record.Version)
		}
		return nil
	}}
	cmd.Flags().StringVar(&opts.cfgPath, "config", opts.cfgPath, "config yaml path")
	return cmd
}

func newAccessKeyRevokeCmd() *cobra.Command {
	opts := accessKeyOptions{cfgPath: "onr.yaml"}
	cmd := &cobra.Command{Use: "revoke", Short: "Revoke a Redis-backed client access key", RunE: func(cmd *cobra.Command, args []string) error {
		client, err := openControlPlane(opts.cfgPath)
		if err != nil {
			return err
		}
		defer func() { _ = client.Close() }()
		if strings.TrimSpace(opts.name) == "" {
			return errors.New("--name is required")
		}
		return client.RevokeAccessKey(context.Background(), opts.name)
	}}
	addAccessKeyFlags(cmd, &opts, false)
	return cmd
}

func newAccessKeyRotateCmd() *cobra.Command {
	opts := accessKeyOptions{cfgPath: "onr.yaml"}
	cmd := &cobra.Command{Use: "rotate", Short: "Rotate a Redis-backed client access key", RunE: func(cmd *cobra.Command, args []string) error {
		client, err := openControlPlane(opts.cfgPath)
		if err != nil {
			return err
		}
		defer func() { _ = client.Close() }()
		if strings.TrimSpace(opts.name) == "" {
			return errors.New("--name is required")
		}
		secret, err := client.RotateAccessKey(context.Background(), opts.name)
		if err != nil {
			return err
		}
		fmt.Printf("name=%s secret=%s\n", opts.name, secret)
		return nil
	}}
	addAccessKeyFlags(cmd, &opts, false)
	return cmd
}

func newAccessKeyMigrateCmd() *cobra.Command {
	var cfgPath, keysPath string
	var dryRun bool
	cmd := &cobra.Command{Use: "migrate", Short: "Migrate access keys from keys.yaml into Redis", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := store.LoadConfigIfExists(cfgPath)
		resolved, _ := store.ResolveDataPaths(cfg, keysPath, "")
		keys, err := keystore.Load(resolved)
		if err != nil {
			return err
		}
		client, err := openControlPlane(cfgPath)
		if err != nil {
			return err
		}
		defer func() { _ = client.Close() }()
		for _, key := range keys.AccessKeys() {
			record := controlplane.AccessKeyRecord{Name: key.Name, SecretHash: client.HashAccessKey(key.Value), Status: "active", SubjectType: "api_key", SubjectID: key.Name, Metadata: map[string]string{"comment": key.Comment}}
			existing, err := client.GetAccessKey(context.Background(), record.Name)
			if err != nil {
				return err
			}
			if existing != nil && existing.SecretHash != record.SecretHash {
				return fmt.Errorf("access key conflict for %q", record.Name)
			}
			if dryRun {
				fmt.Printf("would_migrate name=%s\n", record.Name)
				continue
			}
			if err := client.PutAccessKey(context.Background(), record); err != nil {
				return err
			}
			fmt.Printf("migrated name=%s\n", record.Name)
		}
		return nil
	}}
	cmd.Flags().StringVar(&cfgPath, "config", "onr.yaml", "config yaml path")
	cmd.Flags().StringVar(&keysPath, "from", "", "keys.yaml path")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show migration without writing")
	return cmd
}

func addAccessKeyFlags(cmd *cobra.Command, opts *accessKeyOptions, subject bool) {
	cmd.Flags().StringVar(&opts.cfgPath, "config", opts.cfgPath, "config yaml path")
	cmd.Flags().StringVar(&opts.name, "name", "", "access key name")
	if subject {
		cmd.Flags().StringVar(&opts.subjectType, "subject-type", opts.subjectType, "Meterry subject type")
		cmd.Flags().StringVar(&opts.subjectID, "subject-id", "", "Meterry subject ID")
	}
}

func openControlPlane(cfgPath string) (*controlplane.Client, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	if !cfg.Redis.Enabled {
		return nil, errors.New("redis.enabled must be true")
	}
	return controlplane.New(controlplane.Config{
		Addr: cfg.Redis.Addr, Username: cfg.Redis.Username, Password: cfg.Redis.Password, TLS: cfg.Redis.TLS,
		KeyPrefix: cfg.Redis.KeyPrefix, OperationTimeout: time.Duration(cfg.Redis.OperationTimeoutMs) * time.Millisecond,
		AccessKeyHashSecret: cfg.Redis.AccessKeyHashSecret, BillingStream: cfg.Redis.BillingStream,
		BillingConsumerGroup: cfg.Redis.BillingConsumerGroup, BillingConsumerName: cfg.Redis.BillingConsumerName,
		BillingMaxAttempts: cfg.Redis.BillingMaxAttempts,
	})
}
