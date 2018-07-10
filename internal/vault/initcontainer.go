package vault

import (
	"github.com/spf13/viper"
	k8score "k8s.io/api/core/v1"
	"github.com/nais/naisd/api/app"
	"github.com/hashicorp/go-multierror"
	"fmt"
)

const (
	MountPath             = "/var/run/secrets/naisd.io/vault"
	envVaultAddr          = "NAISD_VAULT_ADDR"
	envInitContainerImage = "NAISD_VAULT_INIT_CONTAINER_IMAGE"
	envVaultAuthPath      = "NAISD_VAULT_AUTH_PATH"
	envVaultKVPath        = "NAISD_VAULT_KV_PATH"
	envVaultEnabled       = "NAISD_VAULT_ENABLED" //temp feature flag

)

type vaultConfig struct {
	vaultAddr          string
	initContainerImage string
	authPath           string
	kvPath             string
}

func (c vaultConfig) validate() (bool, error) {

	var result = &multierror.Error{}

	if len(c.vaultAddr) == 0 {
		multierror.Append(result, fmt.Errorf("vault address not found in environment. Missing %s", envVaultAddr))
	}

	if len(c.initContainerImage) == 0 {
		multierror.Append(result, fmt.Errorf("vault address not found in environment. Missing %s", envInitContainerImage))
	}

	if len(c.authPath) == 0 {
		multierror.Append(result, fmt.Errorf("auth path not found in environment. Missing %s", envVaultAuthPath))
	}

	if len(c.kvPath) == 0 {
		multierror.Append(result, fmt.Errorf("kv path not found in environment. Missing %s", envVaultKVPath))
	}

	return result.ErrorOrNil() == nil, result.ErrorOrNil()

}

func init() {
	viper.BindEnv(envVaultAddr, envVaultAddr)
	viper.BindEnv(envInitContainerImage, envInitContainerImage)
	viper.BindEnv(envVaultAuthPath, envVaultAuthPath)
	viper.BindEnv(envVaultKVPath, envVaultKVPath)

	//temp feature flag. Disable by default
	viper.BindEnv(envVaultEnabled, envVaultEnabled)
	viper.SetDefault(envVaultEnabled, false)

}

type initializer struct {
	spec   app.Spec
	config vaultConfig
}

type Initializer interface {
	AddInitContainer(podSpec *k8score.PodSpec) k8score.PodSpec
}

func Enabled() bool {
	return viper.GetBool(envVaultEnabled)
}

func NewInitializer(spec app.Spec) (Initializer, error) {
	config := vaultConfig{
		vaultAddr:          viper.GetString(envVaultAddr),
		initContainerImage: viper.GetString(envInitContainerImage),
		authPath:           viper.GetString(envVaultAuthPath),
		kvPath:             viper.GetString(envVaultKVPath),
	}

	if ok, err := config.validate(); !ok {
		return nil, err
	}

	return initializer{
		spec:   spec,
		config: config,
	}, nil
}

func (c initializer) AddInitContainer(podSpec *k8score.PodSpec) k8score.PodSpec {

	//Feature flag
	if !Enabled() {
		return *podSpec
	}

	volume, mount := volumeAndMount()

	//Add shared volume to pod
	podSpec.Volumes = append(podSpec.Volumes, volume)

	//Each container in the pod gets the shared volume mounted.
	//Though we should only have one container..
	for _, container := range podSpec.Containers {
		container.VolumeMounts = append(container.VolumeMounts, mount)
	}

	//Finally add init container which also gets the shared volume mounted.
	podSpec.InitContainers = append(podSpec.InitContainers, c.initContainer(mount))

	return *podSpec
}

func volumeAndMount() (k8score.Volume, k8score.VolumeMount) {
	name := "vault-secrets"
	volume := k8score.Volume{
		Name: name,
		VolumeSource: k8score.VolumeSource{
			EmptyDir: &k8score.EmptyDirVolumeSource{
				Medium: k8score.StorageMediumMemory,
			},
		},
	}

	mount := k8score.VolumeMount{
		Name:      name,
		MountPath: MountPath,
	}

	return volume, mount
}

func (c initializer) kvPath() string {
	return c.config.kvPath + "/" + c.spec.Application + "/" + c.spec.Environment
}

func (c initializer) vaultRole() string {
	return c.spec.Application + "/" + c.spec.Environment
}

func (c initializer) initContainer(mount k8score.VolumeMount) k8score.Container {
	return k8score.Container{
		Name:         "vks",
		VolumeMounts: []k8score.VolumeMount{mount},
		Image:        c.config.initContainerImage,
		Env: []k8score.EnvVar{
			{
				Name:  "VKS_VAULT_ADDR",
				Value: c.config.vaultAddr,
			},
			{
				Name:  "VKS_AUTH_PATH",
				Value: c.config.authPath,
			},
			{
				Name:  "VKS_KV_PATH",
				Value: c.kvPath(),
			},
			{
				Name:  "VKS_VAULT_ROLE",
				Value: c.vaultRole(),
			},
			{
				Name:  "VKS_SECRET_DEST_PATH",
				Value: MountPath,
			},
		},
	}

}
