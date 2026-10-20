package vector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	analyticsdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/analytics"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Target identifies an operator workload and its main container, never a sidecar.
type Target struct {
	Name      string
	Container string
	AppName   string
	Labels    map[string]string
}

type vectorTemplateLabel struct {
	Key   string
	Value string
}

type vectorTemplateTarget struct {
	Name      string
	Container string
	AppName   string
	Selector  string
	Labels    []vectorTemplateLabel
}

type vectorTemplateData struct {
	Namespace string
	Endpoint  string
	Targets   []vectorTemplateTarget
}

// VectorConfigMap renders the Kubernetes log pipeline for a Project.
func VectorConfigMap(project *ResourceContext, targets []Target) (*corev1.ConfigMap, error) {
	data := buildVectorTemplateData(project.Namespace, analyticsdefaults.URL(project), targets)
	config, err := renderVectorTemplate(data)
	if err != nil {
		return nil, err
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      VectorName(project),
			Namespace: project.Namespace,
			Labels:    VectorLabels(project),
		},
		Data: map[string]string{"vector.yml": config},
	}, nil
}

func buildVectorTemplateData(namespace, endpoint string, targets []Target) vectorTemplateData {
	data := vectorTemplateData{Namespace: namespace, Endpoint: endpoint, Targets: make([]vectorTemplateTarget, 0, len(targets))}
	for _, target := range targets {
		keys := make([]string, 0, len(target.Labels))
		for key := range target.Labels {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		labels := make([]vectorTemplateLabel, 0, len(keys))
		selectors := make([]string, 0, len(keys))
		for _, key := range keys {
			labels = append(labels, vectorTemplateLabel{Key: key, Value: target.Labels[key]})
			selectors = append(selectors, key+"="+target.Labels[key])
		}
		data.Targets = append(data.Targets, vectorTemplateTarget{
			Name: target.Name, Container: target.Container, AppName: target.AppName,
			Selector: strings.Join(selectors, ","), Labels: labels,
		})
	}
	return data
}

func renderVectorTemplate(data vectorTemplateData) (string, error) {
	// JSON strings are valid quoted scalars in YAML and string literals in VRL.
	quote := func(value string) (string, error) {
		encoded, err := json.Marshal(value)
		return string(encoded), err
	}
	tmpl, err := template.New("vector.yml").Funcs(template.FuncMap{"quote": quote}).Parse(assets.VectorTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing vector template: %w", err)
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return "", fmt.Errorf("rendering vector template: %w", err)
	}
	return output.String(), nil
}
