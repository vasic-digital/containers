package scheduler

// GPURequirement expresses a container's GPU needs.
// Attached via ContainerRequirements.GPU (nil = no GPU needed).
type GPURequirement struct {
	// Count is the number of GPUs needed. Zero defaults to 1 when
	// GPURequirement is non-nil.
	Count int
	// MinVRAMMB is the minimum free VRAM per GPU.
	MinVRAMMB int
	// Vendor restricts to a specific vendor ("nvidia"|"amd"|"intel");
	// empty = any.
	Vendor string
	// MinCompute is the minimum CUDA compute capability (e.g. "8.0").
	// Empty = any.
	MinCompute string
	// Capabilities are required flags: "cuda", "nvenc", "tensorrt",
	// "vulkan", "opencl", "rocm".
	Capabilities []string
}
