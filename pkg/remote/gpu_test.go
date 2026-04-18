package remote

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostResources_GPUFieldDefaultsNil(t *testing.T) {
	var r HostResources
	require.Nil(t, r.GPU)
}

func TestGPUDevice_FieldsAccessible(t *testing.T) {
	d := GPUDevice{
		Index: 0, Vendor: "nvidia", Model: "RTX 3060",
		DriverVersion: "535.104.05", VRAMTotalMB: 6144,
		VRAMFreeMB: 5800, UtilPercent: 3,
		CUDASupported: true, CUDAVersion: "12.2",
		ComputeCapability: "8.6",
		NVENCSupported:    true, NVDECSupported: true,
		VulkanSupported: true, OpenCLSupported: true,
		ROCmSupported: false, NVIDIARuntime: true,
	}
	require.Equal(t, "nvidia", d.Vendor)
	require.Equal(t, 6144, d.VRAMTotalMB)
	require.True(t, d.CUDASupported)
}

func TestHostResources_HasGPU(t *testing.T) {
	r := HostResources{GPU: []GPUDevice{{Vendor: "nvidia"}}}
	require.True(t, r.HasGPU())
	require.False(t, (&HostResources{}).HasGPU())
}
