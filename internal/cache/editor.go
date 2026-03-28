package cache

import (
	"os"
	"os/exec"
)

func Edit(val string) string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tempFile, err := os.CreateTemp("", "req2-edit-*.txt")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(val)
	if err != nil {
		panic(err)
	}

	cmd := exec.Command(editor, tempFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		panic(err)
	}
	editedContent, err := os.ReadFile(tempFile.Name())
	if err != nil {
		panic(err)
	}
	val = string(editedContent)

	return val
}
