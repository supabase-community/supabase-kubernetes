package defaults

import (
	"strconv"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// PostgRESTEnvVars resolves the shared Rest and Studio settings, including
// defaults for Projects created before these fields were added to the CRD.
func PostgRESTEnvVars(spec *core.ProjectPostgRESTSpec) []corev1.EnvVar {
	if spec == nil {
		spec = &core.ProjectPostgRESTSpec{}
	}
	return []corev1.EnvVar{
		helper.EnvVar("PGRST_DB_SCHEMAS", ptr.Deref(spec.Schemas, "public,graphql_public")),
		helper.EnvVar("PGRST_DB_MAX_ROWS", strconv.FormatInt(ptr.Deref(spec.MaxRows, 1000), 10)),
		helper.EnvVar("PGRST_DB_EXTRA_SEARCH_PATH", ptr.Deref(spec.ExtraSearchPath, "public")),
	}
}
