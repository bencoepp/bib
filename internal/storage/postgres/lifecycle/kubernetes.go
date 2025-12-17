// Package postgres provides Kubernetes-based PostgreSQL lifecycle management.
package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"bib/internal/storage"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesManager manages PostgreSQL deployment in Kubernetes.
type KubernetesManager struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
	k8sConfig storage.KubernetesConfig
	nodeID    string
	namespace string
	inCluster bool

	// Resource names
	statefulSetName    string
	serviceName        string
	serviceAccountName string
	secretName         string
	pvcName            string
	backupPVCName      string
	networkPolicyName  string

	// Credentials
	credentials *Credentials
}

// NewKubernetesManager creates a new Kubernetes PostgreSQL manager.
func NewKubernetesManager(cfg LifecycleConfig, nodeID string, creds *Credentials) (*KubernetesManager, error) {
	km := &KubernetesManager{
		nodeID:      nodeID,
		k8sConfig:   cfg.Kubernetes,
		credentials: creds,
	}

	// Detect if running in-cluster or out-of-cluster
	config, err := rest.InClusterConfig()
	if err != nil {
		// Not in cluster, try kubeconfig
		km.inCluster = false

		kubeconfigPath := cfg.KubeconfigPath
		if kubeconfigPath == "" {
			home, _ := os.UserHomeDir()
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kubernetes config: %w", err)
		}
	} else {
		km.inCluster = true
	}

	km.config = config

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}
	km.clientset = clientset

	// Determine namespace
	if err := km.determineNamespace(); err != nil {
		return nil, err
	}

	// Set resource names
	shortNodeID := nodeID
	if len(nodeID) > 8 {
		shortNodeID = nodeID[:8]
	}
	km.statefulSetName = fmt.Sprintf("bibd-postgres-%s", shortNodeID)
	km.serviceName = fmt.Sprintf("bibd-postgres-%s", shortNodeID)
	km.serviceAccountName = fmt.Sprintf("bibd-postgres-%s", shortNodeID)
	km.secretName = fmt.Sprintf("bibd-postgres-creds-%s", shortNodeID)
	km.pvcName = fmt.Sprintf("postgres-data-%s", shortNodeID)
	km.backupPVCName = fmt.Sprintf("postgres-backup-%s", shortNodeID)
	km.networkPolicyName = fmt.Sprintf("bibd-postgres-policy-%s", shortNodeID)

	return km, nil
}

// determineNamespace determines the Kubernetes namespace to use.
func (km *KubernetesManager) determineNamespace() error {
	// Use configured namespace if provided
	if km.k8sConfig.Namespace != "" {
		km.namespace = km.k8sConfig.Namespace
		return nil
	}

	// If in-cluster, try to detect from pod
	if km.inCluster {
		nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err == nil {
			km.namespace = string(nsBytes)
			return nil
		}
	}

	// Default to "default"
	km.namespace = "default"
	return nil
}

// ValidatePrerequisites validates that all prerequisites are met.
func (km *KubernetesManager) ValidatePrerequisites(ctx context.Context) error {
	// Check if we can access the namespace
	_, err := km.clientset.CoreV1().Namespaces().Get(ctx, km.namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("cannot access namespace %s: %w", km.namespace, err)
	}

	// Check RBAC permissions
	if err := km.validateRBACPermissions(ctx); err != nil {
		return fmt.Errorf("insufficient RBAC permissions: %w", err)
	}

	// Check if StorageClass is available
	if km.k8sConfig.StorageClassName != "" {
		_, err := km.clientset.StorageV1().StorageClasses().Get(ctx, km.k8sConfig.StorageClassName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("StorageClass %s not found: %w", km.k8sConfig.StorageClassName, err)
		}
	}

	// If using CNPG, check if operator is installed
	if km.k8sConfig.UseCNPG {
		if err := km.validateCNPGOperator(ctx); err != nil {
			return fmt.Errorf("CNPG operator validation failed: %w", err)
		}
	}

	return nil
}

// validateRBACPermissions validates that we have necessary RBAC permissions.
func (km *KubernetesManager) validateRBACPermissions(ctx context.Context) error {
	// Try to list pods (basic permission check)
	_, err := km.clientset.CoreV1().Pods(km.namespace).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("cannot list pods: %w", err)
	}

	// Try to list StatefulSets
	_, err = km.clientset.AppsV1().StatefulSets(km.namespace).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("cannot list StatefulSets: %w", err)
	}

	// Try to list Services
	_, err = km.clientset.CoreV1().Services(km.namespace).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("cannot list Services: %w", err)
	}

	// Try to list Secrets
	_, err = km.clientset.CoreV1().Secrets(km.namespace).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("cannot list Secrets: %w", err)
	}

	return nil
}

// validateCNPGOperator checks if CNPG operator is installed.
func (km *KubernetesManager) validateCNPGOperator(ctx context.Context) error {
	// Check if CNPG CRDs are installed by looking for deployments in cnpg-system namespace
	_, err := km.clientset.AppsV1().Deployments("cnpg-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=cloudnative-pg",
	})
	if err != nil {
		return fmt.Errorf("CNPG operator not found: %w", err)
	}
	return nil
}

// Deploy deploys PostgreSQL to Kubernetes.
func (km *KubernetesManager) Deploy(ctx context.Context) error {
	// Create ServiceAccount and RBAC if needed
	if km.k8sConfig.CreateRBAC {
		if err := km.createServiceAccount(ctx); err != nil {
			return fmt.Errorf("failed to create ServiceAccount: %w", err)
		}
	}

	// Create Secret for credentials
	if err := km.createCredentialsSecret(ctx); err != nil {
		return fmt.Errorf("failed to create credentials Secret: %w", err)
	}

	// Create PVC for data
	if err := km.createDataPVC(ctx); err != nil {
		return fmt.Errorf("failed to create data PVC: %w", err)
	}

	// Create Service
	if err := km.createService(ctx); err != nil {
		return fmt.Errorf("failed to create Service: %w", err)
	}

	// Create NetworkPolicy if enabled
	if km.k8sConfig.NetworkPolicyEnabled {
		if err := km.createNetworkPolicy(ctx); err != nil {
			return fmt.Errorf("failed to create NetworkPolicy: %w", err)
		}
	}

	// Deploy PostgreSQL (StatefulSet or CNPG Cluster)
	if km.k8sConfig.UseCNPG {
		if err := km.deployCNPGCluster(ctx); err != nil {
			return fmt.Errorf("failed to deploy CNPG Cluster: %w", err)
		}
	} else {
		if err := km.deployStatefulSet(ctx); err != nil {
			return fmt.Errorf("failed to deploy StatefulSet: %w", err)
		}
	}

	// Create backup CronJob if enabled
	if km.k8sConfig.BackupEnabled && !km.k8sConfig.UseCNPG {
		if err := km.createBackupCronJob(ctx); err != nil {
			return fmt.Errorf("failed to create backup CronJob: %w", err)
		}
	}

	// Wait for PostgreSQL to be ready
	if err := km.waitForReady(ctx); err != nil {
		return fmt.Errorf("PostgreSQL failed to become ready: %w", err)
	}

	return nil
}

// createServiceAccount creates a ServiceAccount for PostgreSQL.
func (km *KubernetesManager) createServiceAccount(ctx context.Context) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      km.serviceAccountName,
			Namespace: km.namespace,
			Labels:    km.getLabels(),
		},
	}

	_, err := km.clientset.CoreV1().ServiceAccounts(km.namespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Create Role for minimal permissions (if needed)
	// For now, just basic permissions for the pod itself
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      km.serviceAccountName,
			Namespace: km.namespace,
			Labels:    km.getLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list"},
			},
		},
	}

	_, err = km.clientset.RbacV1().Roles(km.namespace).Create(ctx, role, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Create RoleBinding
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      km.serviceAccountName,
			Namespace: km.namespace,
			Labels:    km.getLabels(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      km.serviceAccountName,
				Namespace: km.namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     km.serviceAccountName,
		},
	}

	_, err = km.clientset.RbacV1().RoleBindings(km.namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// createCredentialsSecret creates a Secret with PostgreSQL credentials.
func (km *KubernetesManager) createCredentialsSecret(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      km.secretName,
			Namespace: km.namespace,
			Labels:    km.getLabels(),
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"superuser-password": km.credentials.SuperuserPassword,
			"admin-password":     km.credentials.AdminPassword,
			"scrape-password":    km.credentials.ScrapePassword,
			"query-password":     km.credentials.QueryPassword,
			"transform-password": km.credentials.TransformPassword,
			"audit-password":     km.credentials.AuditPassword,
			"readonly-password":  km.credentials.ReadOnlyPassword,
		},
	}

	_, err := km.clientset.CoreV1().Secrets(km.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// createDataPVC creates a PersistentVolumeClaim for PostgreSQL data.
func (km *KubernetesManager) createDataPVC(ctx context.Context) error {
	storageSize, err := resource.ParseQuantity(km.k8sConfig.StorageSize)
	if err != nil {
		return fmt.Errorf("invalid storage size %s: %w", km.k8sConfig.StorageSize, err)
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      km.pvcName,
			Namespace: km.namespace,
			Labels:    km.getLabels(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageSize,
				},
			},
		},
	}

	if km.k8sConfig.StorageClassName != "" {
		pvc.Spec.StorageClassName = &km.k8sConfig.StorageClassName
	}

	_, err = km.clientset.CoreV1().PersistentVolumeClaims(km.namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// createService creates a Kubernetes Service for PostgreSQL.
func (km *KubernetesManager) createService(ctx context.Context) error {
	// Determine service type
	serviceType := corev1.ServiceTypeClusterIP
	if km.k8sConfig.ServiceType != "" {
		serviceType = corev1.ServiceType(km.k8sConfig.ServiceType)
	} else {
		// Auto-detect: ClusterIP if in-cluster, NodePort if out-of-cluster
		if !km.inCluster {
			serviceType = corev1.ServiceTypeNodePort
		}
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      km.serviceName,
			Namespace: km.namespace,
			Labels:    km.getLabels(),
		},
		Spec: corev1.ServiceSpec{
			Type: serviceType,
			Selector: map[string]string{
				"app":     "bibd-postgres",
				"node-id": km.nodeID,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "postgres",
					Port:       5432,
					TargetPort: intstr.FromInt(5432),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	// Set NodePort if specified
	if serviceType == corev1.ServiceTypeNodePort && km.k8sConfig.NodePort > 0 {
		svc.Spec.Ports[0].NodePort = int32(km.k8sConfig.NodePort)
	}

	_, err := km.clientset.CoreV1().Services(km.namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// createNetworkPolicy creates a NetworkPolicy to restrict access.
func (km *KubernetesManager) createNetworkPolicy(ctx context.Context) error {
	allowedLabels := km.k8sConfig.NetworkPolicyAllowedLabels
	if allowedLabels == nil {
		allowedLabels = map[string]string{"app": "bibd"}
	}

	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      km.networkPolicyName,
			Namespace: km.namespace,
			Labels:    km.getLabels(),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     "bibd-postgres",
					"node-id": km.nodeID,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: allowedLabels,
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: (*corev1.Protocol)(stringPtr("TCP")),
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 5432},
						},
					},
				},
			},
		},
	}

	_, err := km.clientset.NetworkingV1().NetworkPolicies(km.namespace).Create(ctx, np, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// deployStatefulSet deploys PostgreSQL as a StatefulSet.
func (km *KubernetesManager) deployStatefulSet(ctx context.Context) error {
	replicas := int32(1)

	// Build container
	container := km.buildPostgreSQLContainer()

	// Build pod template
	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: km.getPodLabels(),
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: km.serviceAccountName,
			SecurityContext:    km.buildPodSecurityContext(),
			Containers:         []corev1.Container{container},
			Volumes: []corev1.Volume{
				{
					Name: "postgres-data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: km.pvcName,
						},
					},
				},
			},
		},
	}

	// Add anti-affinity if enabled
	if km.k8sConfig.PodAntiAffinity {
		podTemplate.Spec.Affinity = km.buildAntiAffinity()
	}

	// Add tolerations if configured
	if len(km.k8sConfig.Tolerations) > 0 {
		podTemplate.Spec.Tolerations = km.buildTolerations()
	}

	// Add node selector if configured
	if len(km.k8sConfig.NodeSelector) > 0 {
		podTemplate.Spec.NodeSelector = km.k8sConfig.NodeSelector
	}

	// Add priority class if configured
	if km.k8sConfig.PriorityClassName != "" {
		podTemplate.Spec.PriorityClassName = km.k8sConfig.PriorityClassName
	}

	// Add image pull secrets if configured
	if len(km.k8sConfig.ImagePullSecrets) > 0 {
		for _, secret := range km.k8sConfig.ImagePullSecrets {
			podTemplate.Spec.ImagePullSecrets = append(podTemplate.Spec.ImagePullSecrets, corev1.LocalObjectReference{
				Name: secret,
			})
		}
	}

	// Create StatefulSet
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      km.statefulSetName,
			Namespace: km.namespace,
			Labels:    km.getLabels(),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     "bibd-postgres",
					"node-id": km.nodeID,
				},
			},
			ServiceName: km.serviceName,
			Template:    podTemplate,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.StatefulSetUpdateStrategyType(km.k8sConfig.UpdateStrategy),
			},
		},
	}

	_, err := km.clientset.AppsV1().StatefulSets(km.namespace).Create(ctx, sts, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// buildPostgreSQLContainer builds the PostgreSQL container spec.
func (km *KubernetesManager) buildPostgreSQLContainer() corev1.Container {
	container := corev1.Container{
		Name:  "postgres",
		Image: "postgres:16-alpine", // TODO: Make configurable
		Env: []corev1.EnvVar{
			{
				Name: "POSTGRES_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: km.secretName,
						},
						Key: "superuser-password",
					},
				},
			},
			{
				Name:  "POSTGRES_DB",
				Value: "bibd",
			},
			{
				Name:  "PGDATA",
				Value: "/var/lib/postgresql/data/pgdata",
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "postgres",
				ContainerPort: 5432,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "postgres-data",
				MountPath: "/var/lib/postgresql/data",
			},
		},
	}

	// Add resource requests/limits
	container.Resources = km.buildResourceRequirements()

	// Add probes
	if km.k8sConfig.LivenessProbe.Enabled {
		container.LivenessProbe = km.buildLivenessProbe()
	}
	if km.k8sConfig.ReadinessProbe.Enabled {
		container.ReadinessProbe = km.buildReadinessProbe()
	}
	if km.k8sConfig.StartupProbe.Enabled {
		container.StartupProbe = km.buildStartupProbe()
	}

	return container
}

// buildResourceRequirements builds resource requirements.
func (km *KubernetesManager) buildResourceRequirements() corev1.ResourceRequirements {
	reqs := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}

	if km.k8sConfig.Resources.Requests.CPU != "" {
		cpu, _ := resource.ParseQuantity(km.k8sConfig.Resources.Requests.CPU)
		reqs.Requests[corev1.ResourceCPU] = cpu
	}
	if km.k8sConfig.Resources.Requests.Memory != "" {
		mem, _ := resource.ParseQuantity(km.k8sConfig.Resources.Requests.Memory)
		reqs.Requests[corev1.ResourceMemory] = mem
	}
	if km.k8sConfig.Resources.Limits.CPU != "" {
		cpu, _ := resource.ParseQuantity(km.k8sConfig.Resources.Limits.CPU)
		reqs.Limits[corev1.ResourceCPU] = cpu
	}
	if km.k8sConfig.Resources.Limits.Memory != "" {
		mem, _ := resource.ParseQuantity(km.k8sConfig.Resources.Limits.Memory)
		reqs.Limits[corev1.ResourceMemory] = mem
	}

	return reqs
}

// buildLivenessProbe builds the liveness probe.
func (km *KubernetesManager) buildLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"pg_isready", "-U", "postgres"},
			},
		},
		InitialDelaySeconds: km.k8sConfig.LivenessProbe.InitialDelaySeconds,
		PeriodSeconds:       km.k8sConfig.LivenessProbe.PeriodSeconds,
		TimeoutSeconds:      km.k8sConfig.LivenessProbe.TimeoutSeconds,
		SuccessThreshold:    km.k8sConfig.LivenessProbe.SuccessThreshold,
		FailureThreshold:    km.k8sConfig.LivenessProbe.FailureThreshold,
	}
}

// buildReadinessProbe builds the readiness probe.
func (km *KubernetesManager) buildReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"pg_isready", "-U", "postgres"},
			},
		},
		InitialDelaySeconds: km.k8sConfig.ReadinessProbe.InitialDelaySeconds,
		PeriodSeconds:       km.k8sConfig.ReadinessProbe.PeriodSeconds,
		TimeoutSeconds:      km.k8sConfig.ReadinessProbe.TimeoutSeconds,
		SuccessThreshold:    km.k8sConfig.ReadinessProbe.SuccessThreshold,
		FailureThreshold:    km.k8sConfig.ReadinessProbe.FailureThreshold,
	}
}

// buildStartupProbe builds the startup probe.
func (km *KubernetesManager) buildStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"pg_isready", "-U", "postgres"},
			},
		},
		InitialDelaySeconds: km.k8sConfig.StartupProbe.InitialDelaySeconds,
		PeriodSeconds:       km.k8sConfig.StartupProbe.PeriodSeconds,
		TimeoutSeconds:      km.k8sConfig.StartupProbe.TimeoutSeconds,
		SuccessThreshold:    km.k8sConfig.StartupProbe.SuccessThreshold,
		FailureThreshold:    km.k8sConfig.StartupProbe.FailureThreshold,
	}
}

// buildPodSecurityContext builds the pod security context.
func (km *KubernetesManager) buildPodSecurityContext() *corev1.PodSecurityContext {
	psc := &corev1.PodSecurityContext{}

	if km.k8sConfig.SecurityContext.RunAsNonRoot {
		psc.RunAsNonRoot = &km.k8sConfig.SecurityContext.RunAsNonRoot
	}
	if km.k8sConfig.SecurityContext.RunAsUser > 0 {
		psc.RunAsUser = &km.k8sConfig.SecurityContext.RunAsUser
	}
	if km.k8sConfig.SecurityContext.RunAsGroup > 0 {
		psc.RunAsGroup = &km.k8sConfig.SecurityContext.RunAsGroup
	}
	if km.k8sConfig.SecurityContext.FSGroup > 0 {
		psc.FSGroup = &km.k8sConfig.SecurityContext.FSGroup
	}
	if km.k8sConfig.SecurityContext.FSGroupChangePolicy != "" {
		policy := corev1.PodFSGroupChangePolicy(km.k8sConfig.SecurityContext.FSGroupChangePolicy)
		psc.FSGroupChangePolicy = &policy
	}
	if km.k8sConfig.SecurityContext.SeccompProfile != "" {
		psc.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}

	return psc
}

// buildAntiAffinity builds pod anti-affinity rules.
func (km *KubernetesManager) buildAntiAffinity() *corev1.Affinity {
	labels := km.k8sConfig.PodAntiAffinityLabels
	if labels == nil {
		labels = map[string]string{"app": "bibd"}
	}

	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
	}
}

// buildTolerations builds pod tolerations.
func (km *KubernetesManager) buildTolerations() []corev1.Toleration {
	tolerations := []corev1.Toleration{}
	for _, t := range km.k8sConfig.Tolerations {
		toleration := corev1.Toleration{
			Key:      t.Key,
			Operator: corev1.TolerationOperator(t.Operator),
			Value:    t.Value,
			Effect:   corev1.TaintEffect(t.Effect),
		}
		if t.TolerationSeconds != nil {
			toleration.TolerationSeconds = t.TolerationSeconds
		}
		tolerations = append(tolerations, toleration)
	}
	return tolerations
}

// createBackupCronJob creates a CronJob for PostgreSQL backups.
func (km *KubernetesManager) createBackupCronJob(ctx context.Context) error {
	// Create backup PVC if not using S3
	if !km.k8sConfig.BackupToS3 {
		if err := km.createBackupPVC(ctx); err != nil {
			return err
		}
	}

	// Build backup command
	backupCmd := km.buildBackupCommand()

	cronjob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-backup", km.statefulSetName),
			Namespace: km.namespace,
			Labels:    km.getLabels(),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: km.k8sConfig.BackupSchedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:    "backup",
									Image:   "postgres:16-alpine",
									Command: []string{"/bin/sh", "-c", backupCmd},
									Env: []corev1.EnvVar{
										{
											Name: "PGPASSWORD",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: km.secretName,
													},
													Key: "superuser-password",
												},
											},
										},
										{
											Name:  "PGHOST",
											Value: km.serviceName,
										},
										{
											Name:  "PGUSER",
											Value: "postgres",
										},
										{
											Name:  "PGDATABASE",
											Value: "bibd",
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "backup",
											MountPath: "/backup",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "backup",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: km.backupPVCName,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := km.clientset.BatchV1().CronJobs(km.namespace).Create(ctx, cronjob, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// createBackupPVC creates a PVC for backups.
func (km *KubernetesManager) createBackupPVC(ctx context.Context) error {
	storageSize, err := resource.ParseQuantity(km.k8sConfig.BackupStorageSize)
	if err != nil {
		return fmt.Errorf("invalid backup storage size %s: %w", km.k8sConfig.BackupStorageSize, err)
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      km.backupPVCName,
			Namespace: km.namespace,
			Labels:    km.getLabels(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageSize,
				},
			},
		},
	}

	if km.k8sConfig.StorageClassName != "" {
		pvc.Spec.StorageClassName = &km.k8sConfig.StorageClassName
	}

	_, err = km.clientset.CoreV1().PersistentVolumeClaims(km.namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// buildBackupCommand builds the backup command.
func (km *KubernetesManager) buildBackupCommand() string {
	// Rotate old backups based on retention
	rotateCmd := fmt.Sprintf("ls -t /backup/backup-*.sql.gz 2>/dev/null | tail -n +%d | xargs -r rm", km.k8sConfig.BackupRetention+1)

	// Create new backup
	timestamp := "$(date +%%Y%%m%%d-%%H%%M%%S)"
	backupCmd := fmt.Sprintf("pg_dump -Fc | gzip > /backup/backup-%s.sql.gz", timestamp)

	return fmt.Sprintf("%s && %s", rotateCmd, backupCmd)
}

// deployCNPGCluster deploys PostgreSQL using CloudNativePG operator.
func (km *KubernetesManager) deployCNPGCluster(ctx context.Context) error {
	// TODO: Implement CNPG Cluster deployment
	// This requires defining the CNPG CRD types or using dynamic client
	return fmt.Errorf("CNPG deployment not yet implemented")
}

// waitForReady waits for PostgreSQL to be ready.
func (km *KubernetesManager) waitForReady(ctx context.Context) error {
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for PostgreSQL to be ready")
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Check if StatefulSet is ready
			sts, err := km.clientset.AppsV1().StatefulSets(km.namespace).Get(ctx, km.statefulSetName, metav1.GetOptions{})
			if err != nil {
				continue
			}

			if sts.Status.ReadyReplicas > 0 {
				return nil
			}
		}
	}
}

// Cleanup removes Kubernetes resources.
func (km *KubernetesManager) Cleanup(ctx context.Context) error {
	// Delete CronJob
	km.clientset.BatchV1().CronJobs(km.namespace).Delete(ctx, fmt.Sprintf("%s-backup", km.statefulSetName), metav1.DeleteOptions{})

	if km.k8sConfig.DeleteOnCleanup {
		// Delete StatefulSet
		km.clientset.AppsV1().StatefulSets(km.namespace).Delete(ctx, km.statefulSetName, metav1.DeleteOptions{})

		// Delete Service
		km.clientset.CoreV1().Services(km.namespace).Delete(ctx, km.serviceName, metav1.DeleteOptions{})

		// Delete NetworkPolicy
		km.clientset.NetworkingV1().NetworkPolicies(km.namespace).Delete(ctx, km.networkPolicyName, metav1.DeleteOptions{})

		// Delete Secret
		km.clientset.CoreV1().Secrets(km.namespace).Delete(ctx, km.secretName, metav1.DeleteOptions{})

		// Delete PVCs
		km.clientset.CoreV1().PersistentVolumeClaims(km.namespace).Delete(ctx, km.pvcName, metav1.DeleteOptions{})
		km.clientset.CoreV1().PersistentVolumeClaims(km.namespace).Delete(ctx, km.backupPVCName, metav1.DeleteOptions{})

		// Delete RBAC resources
		if km.k8sConfig.CreateRBAC {
			km.clientset.RbacV1().RoleBindings(km.namespace).Delete(ctx, km.serviceAccountName, metav1.DeleteOptions{})
			km.clientset.RbacV1().Roles(km.namespace).Delete(ctx, km.serviceAccountName, metav1.DeleteOptions{})
			km.clientset.CoreV1().ServiceAccounts(km.namespace).Delete(ctx, km.serviceAccountName, metav1.DeleteOptions{})
		}
	} else {
		// Scale StatefulSet to 0
		sts, err := km.clientset.AppsV1().StatefulSets(km.namespace).Get(ctx, km.statefulSetName, metav1.GetOptions{})
		if err == nil {
			zero := int32(0)
			sts.Spec.Replicas = &zero
			km.clientset.AppsV1().StatefulSets(km.namespace).Update(ctx, sts, metav1.UpdateOptions{})
		}
	}

	return nil
}

// GetConnectionInfo returns connection information for the PostgreSQL instance.
func (km *KubernetesManager) GetConnectionInfo(ctx context.Context) (string, int, error) {
	svc, err := km.clientset.CoreV1().Services(km.namespace).Get(ctx, km.serviceName, metav1.GetOptions{})
	if err != nil {
		return "", 0, err
	}

	// Return service DNS name and port
	host := fmt.Sprintf("%s.%s.svc.cluster.local", km.serviceName, km.namespace)
	port := 5432

	// If NodePort, we need to get the node IP
	if svc.Spec.Type == corev1.ServiceTypeNodePort {
		// Get first node IP
		nodes, err := km.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
		if err != nil {
			return "", 0, err
		}
		if len(nodes.Items) > 0 {
			for _, addr := range nodes.Items[0].Status.Addresses {
				if addr.Type == corev1.NodeExternalIP || addr.Type == corev1.NodeInternalIP {
					host = addr.Address
					break
				}
			}
		}
		if len(svc.Spec.Ports) > 0 {
			port = int(svc.Spec.Ports[0].NodePort)
		}
	}

	return host, port, nil
}

// getLabels returns standard labels for resources.
func (km *KubernetesManager) getLabels() map[string]string {
	labels := map[string]string{
		"app":        "bibd-postgres",
		"node-id":    km.nodeID,
		"component":  "database",
		"managed-by": "bibd",
	}

	// Add custom labels
	for k, v := range km.k8sConfig.Labels {
		labels[k] = v
	}

	return labels
}

// getPodLabels returns labels for pods.
func (km *KubernetesManager) getPodLabels() map[string]string {
	labels := km.getLabels()
	labels["app"] = "bibd-postgres"
	labels["node-id"] = km.nodeID
	return labels
}

// Helper function to get string pointer
func stringPtr(s string) *string {
	return &s
}
