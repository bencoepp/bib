package checks

import "bib/internal/capcheck"

func AllCheckers() []capcheck.Checker {
	return []capcheck.Checker{
		ContainerRuntimeChecker{},
		KubernetesConfigChecker{},
		InternetAccessChecker{HttpUrl: "https://www.google.com/generate_204"},
		ResourcesChecker{},

		GPUChecker{},
		DiskStorageChecker{},
		DiskPerformanceChecker{},
		NetworkReachabilityChecker{},
		DNSResolverChecker{},
		ProxyChecker{},
		TLSPKIChecker{},
		TimeNTPChecker{},
		OSKernelFeaturesChecker{},
		VirtualizationChecker{},
		CPUFeaturesChecker{},
		MemoryCharacteristicsChecker{},
		SystemLimitsChecker{},
		SecurityPostureChecker{},
		CLIToolchainChecker{},
		LanguageRuntimesChecker{},
		ContainerEcosystemChecker{},
		KubernetesDeepChecker{},
		CloudEnvironmentChecker{},
		SourceControlConnectivityChecker{},
		VPNChecker{},
		TempCacheDirsChecker{},
		LocaleEnvironmentChecker{},
		SecretsKeyStoresChecker{},
		PerformanceHealthChecker{},
		ComplianceChecker{},
	}
}
