## Resolving image mounting issue

1. Troubleshoot using `strace`

```go
func mountImage(pathToBaseImage string) {
    // Must open as read write
	imageFile, err := os.OpenFile(pathToBaseImage, os.O_RDWR, 0644)
}

// https://github.com/paultag/go-loopback
// loopback.go
// Should PR?
func NextLoopDevice() (*os.File, error) {
	loopInt, err := nextUnallocatedLoop()
	if err != nil {
		return nil, err
	}
    // from 
	return os.Open(fmt.Sprintf("/dev/loop%d", loopInt))

    // to
    // Must also open loopback as O_RDWR
	return os.OpenFile(fmt.Sprintf("/dev/loop%d", loopInt), os.O_RDWR, 0644)
}
```

## Learning lesson

1. Loopback device must be RW
2. Image must open as RW
3. What is `FileMode`?