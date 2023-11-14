package controller

import (
	"context"
	"github.com/go-logr/logr"
	"os"
	"os/exec"
	"time"
)

func GiB2Bytes(datavolume int) int {
	return datavolume * 1024 * 1024 * 1024
}

func LogInfoInterval(ctx context.Context, interval int, msg string) {
	logger, _ := logr.FromContext(ctx)
	if interval < 5 {
		interval = 5
	}
	if time.Now().Second()%interval == 0 {
		logger.Info(msg)
	}
}

func LogInfo(ctx context.Context, msg string) {
	logger, _ := logr.FromContext(ctx)
	logger.Info(msg)
}

func HelmInstallCmd(chartRepo string, releaseName string, chartPath string, chartVersion string, ns string, params map[string]string) error {
	//argString := " --repo " + chartRepo + " " + chartPath + " --version=" + chartVersion + " --namespace " + ns
	argString := " " + chartPath + " --version=" + chartVersion + " --namespace " + ns
	for k, v := range params {
		argString += " --set " + k + "=" + v
	}

	cmd := exec.Command("helm", "install", releaseName, argString)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func HelmUnInstallCmd(releaseName string, ns string) error {
	cmd := exec.Command("helm", "uninstall", releaseName, "-n", ns)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
