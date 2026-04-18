package remote

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const sampleNvidiaSmi = `0, NVIDIA GeForce RTX 3060, 535.104.05, 6144, 5800, 3, 8.6
`

func TestParseNvidiaSmi_OneGPU(t *testing.T) {
	devs, err := ParseNvidiaSmi(sampleNvidiaSmi)
	require.NoError(t, err)
	require.Len(t, devs, 1)
	d := devs[0]
	require.Equal(t, 0, d.Index)
	require.Equal(t, "nvidia", d.Vendor)
	require.Equal(t, "NVIDIA GeForce RTX 3060", d.Model)
	require.Equal(t, "535.104.05", d.DriverVersion)
	require.Equal(t, 6144, d.VRAMTotalMB)
	require.Equal(t, 5800, d.VRAMFreeMB)
	require.Equal(t, 3, d.UtilPercent)
	require.Equal(t, "8.6", d.ComputeCapability)
	require.True(t, d.CUDASupported)
}

func TestParseNvidiaSmi_Empty(t *testing.T) {
	devs, err := ParseNvidiaSmi("")
	require.NoError(t, err)
	require.Empty(t, devs)
}

func TestParseNvidiaSmi_Malformed(t *testing.T) {
	_, err := ParseNvidiaSmi("not a csv row")
	require.Error(t, err)
}

const sampleRocmSmi = `GPU[0]		: Card series: Radeon RX 6800
GPU[0]		: Card model: 0x73bf
GPU[0]		: VRAM Total Memory (B): 17163091968
GPU[0]		: VRAM Total Used Memory (B): 524288
`

func TestParseRocmSmi_OneGPU(t *testing.T) {
	devs, err := ParseRocmSmi(sampleRocmSmi)
	require.NoError(t, err)
	require.Len(t, devs, 1)
	require.Equal(t, "amd", devs[0].Vendor)
	require.True(t, devs[0].ROCmSupported)
}
