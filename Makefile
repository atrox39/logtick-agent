UNAME_S := $(shell uname -s)

# Detecta si el sistema contiene 'NT-' (Windows)
IS_WINDOWS := $(findstring NT-,$(UNAME_S))

ifeq ($(OS),Windows_NT) # cmd o powershell con make
	EXT := .exe
	OUT := agent.exe
else ifneq ($(IS_WINDOWS),) # Git Bash / MSYS2 / Cygwin
	EXT := .exe
	OUT := agent.exe
else
	EXT :=
	OUT := agent
endif

build:
	@echo "Detected uname -s: $(UNAME_S)"
	@echo "Detected OS: $(OS)"
	@echo "Building for output: $(OUT)"
	go build -o $(OUT) .
