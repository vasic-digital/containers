package remote

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

// ParseNvidiaSmi parses the output of:
//
//	nvidia-smi --query-gpu=index,name,driver_version,memory.total,
//	           memory.free,utilization.gpu,compute_cap \
//	           --format=csv,noheader,nounits
//
// into GPUDevice records. Returns an error on any malformed row.
// An empty input returns an empty slice + nil error.
func ParseNvidiaSmi(raw string) ([]GPUDevice, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	r := csv.NewReader(strings.NewReader(raw))
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse nvidia-smi csv: %w", err)
	}
	out := make([]GPUDevice, 0, len(rows))
	for i, row := range rows {
		if len(row) < 7 {
			return nil, fmt.Errorf(
				"nvidia-smi row %d: expected 7 cols, got %d", i, len(row))
		}
		idx, err := strconv.Atoi(strings.TrimSpace(row[0]))
		if err != nil {
			return nil, fmt.Errorf(
				"nvidia-smi row %d index: %w", i, err)
		}
		vramTotal, err := strconv.Atoi(strings.TrimSpace(row[3]))
		if err != nil {
			return nil, fmt.Errorf(
				"nvidia-smi row %d vram_total: %w", i, err)
		}
		vramFree, err := strconv.Atoi(strings.TrimSpace(row[4]))
		if err != nil {
			return nil, fmt.Errorf(
				"nvidia-smi row %d vram_free: %w", i, err)
		}
		util, err := strconv.Atoi(strings.TrimSpace(row[5]))
		if err != nil {
			return nil, fmt.Errorf(
				"nvidia-smi row %d util: %w", i, err)
		}
		out = append(out, GPUDevice{
			Index:             idx,
			Vendor:            "nvidia",
			Model:             strings.TrimSpace(row[1]),
			DriverVersion:     strings.TrimSpace(row[2]),
			VRAMTotalMB:       vramTotal,
			VRAMFreeMB:        vramFree,
			UtilPercent:       util,
			ComputeCapability: strings.TrimSpace(row[6]),
			CUDASupported:     true,
			NVENCSupported:    true,
			NVDECSupported:    true,
			VulkanSupported:   true,
			OpenCLSupported:   true,
		})
	}
	return out, nil
}

// ParseRocmSmi parses a minimal subset of rocm-smi's default text
// output, extracting vendor + model + VRAM per GPU index.
func ParseRocmSmi(raw string) ([]GPUDevice, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	gpus := make(map[int]*GPUDevice)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "GPU[") {
			continue
		}
		// Example: GPU[0]		: Card series: Radeon RX 6800
		close := strings.Index(line, "]")
		if close < 0 {
			continue
		}
		idx, err := strconv.Atoi(line[4:close])
		if err != nil {
			continue
		}
		dev, ok := gpus[idx]
		if !ok {
			dev = &GPUDevice{
				Index:           idx,
				Vendor:          "amd",
				ROCmSupported:   true,
				VulkanSupported: true,
				OpenCLSupported: true,
			}
			gpus[idx] = dev
		}
		rest := strings.TrimSpace(line[close+1:])
		rest = strings.TrimPrefix(rest, ":")
		rest = strings.TrimSpace(rest)
		switch {
		case strings.HasPrefix(rest, "Card series:"):
			dev.Model = strings.TrimSpace(
				strings.TrimPrefix(rest, "Card series:"))
		case strings.HasPrefix(rest, "VRAM Total Memory (B):"):
			val := strings.TrimSpace(
				strings.TrimPrefix(rest, "VRAM Total Memory (B):"))
			if n, err := strconv.ParseUint(val, 10, 64); err == nil {
				dev.VRAMTotalMB = int(n / (1024 * 1024))
			}
		}
	}
	out := make([]GPUDevice, 0, len(gpus))
	for _, d := range gpus {
		out = append(out, *d)
	}
	return out, nil
}
