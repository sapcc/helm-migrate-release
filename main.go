// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig string
	to         string
	namespace  string
	maxHist    int
)

func main() {
	flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "path to your kubeconfig file")
	flag.StringVar(&to, "to", "", "kind of resource to migrate to (configmap or secret)")
	flag.StringVar(&namespace, "namespace", "default", "namespace containing releases to migrate")
	flag.IntVar(&maxHist, "max", 1, "history length to migrate")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Migrate Helm releases from $HELM_DRIVER to other drivers.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [flags] subprogram [args]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  release <release name>\n")
		fmt.Fprintf(os.Stderr, "  namespace\n")
		fmt.Fprintf(os.Stderr, "  all\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	subcommands := flag.Arg(0)
	if subcommands == "" {
		fmt.Println("subprogram is required")
		os.Exit(1)
	}
	migrator, err := NewMigrator(kubeconfig, namespace)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	switch subcommands {
	case "release":
		releaseName := flag.Arg(1)
		if releaseName == "" {
			fmt.Println("release name is required")
			os.Exit(1)
		}
		err = migrator.migrateRelease(releaseName, namespace)
	case "namespace":
		err = migrator.migrateNamespace(namespace)
	case "all":
		err = migrator.migrateAll()
	default:
		err = fmt.Errorf("unknown subprogram %s", subcommands)
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type Migrator struct {
	clientset *kubernetes.Clientset
	actionCfg *action.Configuration
}

func NewMigrator(kubeconfig, namespace string) (*Migrator, error) {
	kubecfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(kubecfg)
	if err != nil {
		return nil, err
	}
	var cfg action.Configuration
	err = cfg.Init(kube.GetConfig(kubeconfig, "", ""), namespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Printf("%s\n", fmt.Sprintf(format, v...))
	})
	if err != nil {
		return nil, err
	}
	return &Migrator{
		clientset: clientset,
		actionCfg: &cfg,
	}, nil
}

func (m *Migrator) migrateRelease(releaseName, namespace string) error {
	var helmStorage *storage.Storage
	switch to {
	case "configmap", "configmaps":
		helmStorage = storage.Init(driver.NewConfigMaps(m.clientset.CoreV1().ConfigMaps(namespace)))
	case "secret", "secrets":
		helmStorage = storage.Init(driver.NewSecrets(m.clientset.CoreV1().Secrets(namespace)))
	default:
		return fmt.Errorf("unknown resource type %s", to)
	}
	histCmd := action.NewHistory(m.actionCfg)
	histCmd.Max = maxHist
	hist, err := histCmd.Run(releaseName)
	if err != nil {
		return err
	}
	failed := false
	for _, release := range hist {
		err = helmStorage.Create(release)
		if err != nil {
			failed = true
			fmt.Printf("failed to migrate release %s version %d,: %s\n", releaseName, release.Version, err)
			continue
		}
		_, err = m.actionCfg.Releases.Delete(releaseName, release.Version)
		if err != nil {
			failed = true
			fmt.Printf("failed to delete release %s version %d: %s\n", releaseName, release.Version, err)
			continue
		}
		fmt.Printf("migrated release %s version %d\n", releaseName, release.Version)
	}
	if failed {
		return fmt.Errorf("failed to migrate release %s", releaseName)
	}
	return nil
}

func (m *Migrator) migrateNamespace(namespace string) error {
	releases, err := action.NewList(m.actionCfg).Run()
	if err != nil {
		return err
	}
	for _, release := range releases {
		if release.Namespace == namespace {
			err = m.migrateRelease(release.Name, namespace)
			if err != nil {
				fmt.Println(err)
				continue
			}
		}
	}
	return nil
}

func (m *Migrator) migrateAll() error {
	listCmd := action.NewList(m.actionCfg)
	listCmd.AllNamespaces = true
	releases, err := listCmd.Run()
	if err != nil {
		return err
	}
	for _, release := range releases {
		err = m.migrateRelease(release.Name, release.Namespace)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}
	return nil
}
