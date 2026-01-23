package data_manager

import (
	"fmt"
	"os"
	"os/exec"
)

type PythonBridge struct {
	pythonPath      string
	marketCapScript string
}

func NewPythonBridge(pythonPath, marketCapScript string) *PythonBridge {
	return &PythonBridge{
		pythonPath:      pythonPath,
		marketCapScript: marketCapScript,
	}
}

func (b *PythonBridge) UpdateMarketCaps() error {
	cmd := exec.Command(b.pythonPath, b.marketCapScript, "update")
	// Set data dir env var
	cmd.Env = append(os.Environ(), fmt.Sprintf("DATA_DIR=%s", "data")) // Simplified for now
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("python script failed: %v, output: %s", err, string(output))
	}
	fmt.Printf("Python script output: %s\n", string(output))
	return nil
}
