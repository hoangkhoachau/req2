package utils

import (
	"os"
	"os/exec"
)

func Edit(val string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tempFile, err := os.CreateTemp("", "req2-edit-*.txt")
	if err != nil {
		return val, err
	}
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(val)
	tempFile.Close()
	if err != nil {
		return val, err
	}

	cmd := exec.Command(editor, tempFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return val, err
	}
	editedContent, err := os.ReadFile(tempFile.Name())
	if err != nil {
		return val, err
	}
	val = string(editedContent)

	return val, nil
}
