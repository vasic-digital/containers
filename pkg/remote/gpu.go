package remote

// GPUDevice describes one GPU accelerator visible to a host.
// Populated by ProbeGPU (see probe_gpu.go) or from env-config
// labels when probing is disabled.
type GPUDevice struct {
	Index             int    `json:"index"`
	Vendor            string `json:"vendor"`
	Model             string `json:"model"`
	DriverVersion     string `json:"driver_version"`
	VRAMTotalMB       int    `json:"vram_total_mb"`
	VRAMFreeMB        int    `json:"vram_free_mb"`
	UtilPercent       int    `json:"util_percent"`
	CUDASupported     bool   `json:"cuda_supported"`
	CUDAVersion       string `json:"cuda_version,omitempty"`
	ComputeCapability string `json:"compute_capability,omitempty"`
	NVENCSupported    bool   `json:"nvenc_supported"`
	NVDECSupported    bool   `json:"nvdec_supported"`
	VulkanSupported   bool   `json:"vulkan_supported"`
	OpenCLSupported   bool   `json:"opencl_supported"`
	ROCmSupported     bool   `json:"rocm_supported"`
	NVIDIARuntime     bool   `json:"nvidia_runtime"`
}

// HasGPU reports whether this snapshot contains at least one GPU.
func (r *HostResources) HasGPU() bool { return len(r.GPU) > 0 }
