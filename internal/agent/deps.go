package agent

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Python standard library module whitelist (don't try to pip install these)
var pythonStdLib = map[string]bool{
	"abc": true, "argparse": true, "array": true, "ast": true, "asynchat": true,
	"asyncio": true, "asyncore": true, "base64": true, "binascii": true, "binhex": true,
	"bisect": true, "builtins": true, "bz2": true, "calendar": true, "cgi": true,
	"cgitb": true, "chunk": true, "cmath": true, "cmd": true, "code": true,
	"codecs": true, "codeop": true, "collections": true, "colorsys": true, "compileall": true,
	"concurrent": true, "configparser": true, "contextlib": true, "contextvars": true,
	"copy": true, "copyreg": true, "cProfile": true, "crypt": true, "csv": true,
	"ctypes": true, "curses": true, "dataclasses": true, "datetime": true, "dbm": true,
	"decimal": true, "difflib": true, "dis": true, "distutils": true, "doctest": true,
	"email": true, "encodings": true, "ensurepip": true, "enum": true, "errno": true,
	"faulthandler": true, "fcntl": true, "filecmp": true, "fileinput": true, "fnmatch": true,
	"fractions": true, "ftplib": true, "functools": true, "gc": true, "getopt": true,
	"getpass": true, "gettext": true, "glob": true, "graphlib": true, "gzip": true,
	"hashlib": true, "heapq": true, "hmac": true, "html": true, "http": true,
	"imaplib": true, "imghdr": true, "imp": true, "importlib": true, "inspect": true,
	"io": true, "ipaddress": true, "itertools": true, "json": true, "keyword": true,
	"linecache": true, "locale": true, "logging": true, "lzma": true, "mailbox": true,
	"mailcap": true, "marshal": true, "math": true, "mimetypes": true, "mmap": true,
	"modulefinder": true, "msilib": true, "msvcrt": true, "multiprocessing": true,
	"netrc": true, "nntplib": true, "numbers": true, "operator": true, "optparse": true,
	"os": true, "ossaudiodev": true, "pathlib": true, "pdb": true, "pickle": true,
	"pickletools": true, "pipes": true, "pkgutil": true, "platform": true, "plistlib": true,
	"poplib": true, "posix": true, "posixpath": true, "pprint": true, "profile": true,
	"pstats": true, "pty": true, "pwd": true, "py_compile": true, "pyclbr": true,
	"pydoc": true, "queue": true, "quopri": true, "random": true, "re": true,
	"readline": true, "reprlib": true, "resource": true, "rlcompleter": true, "runpy": true,
	"sched": true, "secrets": true, "select": true, "selectors": true, "shelve": true,
	"shlex": true, "shutil": true, "signal": true, "site": true, "smtpd": true,
	"smtplib": true, "sndhdr": true, "socket": true, "socketserver": true, "spwd": true,
	"sqlite3": true, "ssl": true, "stat": true, "statistics": true, "string": true,
	"stringprep": true, "struct": true, "subprocess": true, "sunau": true, "symbol": true,
	"symtable": true, "sys": true, "sysconfig": true, "syslog": true, "tabnanny": true,
	"tarfile": true, "telnetlib": true, "tempfile": true, "termios": true, "test": true,
	"textwrap": true, "threading": true, "time": true, "timeit": true, "tkinter": true,
	"token": true, "tokenize": true, "trace": true, "traceback": true, "tracemalloc": true,
	"tty": true, "turtle": true, "turtledemo": true, "types": true, "typing": true,
	"unicodedata": true, "unittest": true, "urllib": true, "uu": true, "uuid": true,
	"venv": true, "warnings": true, "wave": true, "weakref": true, "webbrowser": true,
	"winreg": true, "winsound": true, "wsgiref": true, "xdrlib": true, "xml": true,
	"xmlrpc": true, "zipapp": true, "zipfile": true, "zipimport": true, "zlib": true,
	"zoneinfo": true,
}

// Special alias mappings for python package names
var pythonAliasMap = map[string]string{
	"PIL":           "pillow",
	"yaml":          "pyyaml",
	"bs4":           "beautifulsoup4",
	"dotenv":        "python-dotenv",
	"telegram":      "python-telegram-bot",
	"cv2":           "opencv-python",
	"sklearn":       "scikit-learn",
	"dateutil":      "python-dateutil",
	"jwt":           "pyjwt",
	"psycopg2":      "psycopg2-binary",
	"Crypto":        "pycryptodome",
	"serial":        "pyserial",
	"OpenSSL":       "pyopenssl",
	"magic":         "python-magic",
	"jose":          "python-jose",
	"multipart":     "python-multipart",
	"magic_filter":  "magic-filter",
	"aiogram":       "aiogram",
}

// Node.js built-in modules
var nodeBuiltIns = map[string]bool{
	"assert": true, "async_hooks": true, "buffer": true, "child_process": true,
	"cluster": true, "console": true, "constants": true, "crypto": true,
	"dgram": true, "diagnostics_channel": true, "dns": true, "domain": true,
	"events": true, "fs": true, "fs/promises": true, "http": true, "http2": true,
	"https": true, "inspector": true, "module": true, "net": true, "os": true,
	"path": true, "path/posix": true, "path/win32": true, "perf_hooks": true,
	"process": true, "punycode": true, "querystring": true, "readline": true,
	"repl": true, "stream": true, "stream/promises": true, "stream/consumers": true,
	"stream/web": true, "string_decoder": true, "timers": true, "timers/promises": true,
	"tls": true, "trace_events": true, "tty": true, "url": true, "util": true,
	"util/types": true, "v8": true, "vm": true, "wasi": true, "worker_threads": true,
	"zlib": true, "node:test": true,
}

// CommandReporter reports active background commands
type CommandReporter func(*exec.Cmd)

// EnsureDependencies inspects code and installs missing dependencies for python, nodejs, or golang
func EnsureDependencies(ctx context.Context, lang, code, workDir string, logWriter io.Writer, onCmd CommandReporter) error {
	switch strings.ToLower(lang) {
	case "python", "py":
		return ensurePythonDeps(ctx, code, logWriter, onCmd)
	case "nodejs", "node", "javascript", "js":
		return ensureNodeDeps(ctx, code, workDir, logWriter, onCmd)
	case "golang", "go":
		return ensureGoDeps(ctx, workDir, logWriter, onCmd)
	}
	return nil
}

// findExecutable searches for available binary names or paths
func findExecutable(names ...string) string {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return ""
}

func ensurePythonDeps(ctx context.Context, code string, logWriter io.Writer, onCmd CommandReporter) error {
	pythonBin := findExecutable("python3", "python", "/usr/bin/python3", "/usr/bin/python", "/usr/local/bin/python3")
	if pythonBin == "" {
		fmt.Fprintf(logWriter, "[Node Agent] Предупреждение: python не найден в PATH!\n")
		return nil
	}

	pipBin := findExecutable("pip3", "pip", "/usr/bin/pip3", "/usr/bin/pip", "/usr/local/bin/pip3")

	// Detect packages imported in Python code
	// Match: import xyz, import xyz.abc, from xyz import ...
	reImport := regexp.MustCompile(`(?m)^\s*(?:import|from)\s+([a-zA-Z0-9_]+)`)
	matches := reImport.FindAllStringSubmatch(code, -1)
	
	detected := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			mod := match[1]
			if !pythonStdLib[mod] {
				detected[mod] = true
			}
		}
	}

	for mod := range detected {
		// Check if already installed
		checkCmd := exec.CommandContext(ctx, pythonBin, "-c", fmt.Sprintf("import %s", mod))
		if err := checkCmd.Run(); err == nil {
			// Module already present
			continue
		}

		// Needs install
		pkgName := mod
		if alias, exists := pythonAliasMap[mod]; exists {
			pkgName = alias
		}

		if pipBin == "" {
			fmt.Fprintf(logWriter, "[Node Agent] Не найден pip для установки модуля: %s\n", pkgName)
			continue
		}

		fmt.Fprintf(logWriter, "[Node Agent] 📦 Обнаружена отсутствующая зависимость Python '%s'. Установка через pip...\n", pkgName)
		installCmd := exec.CommandContext(ctx, pipBin, "install", "--quiet", pkgName)
		if onCmd != nil {
			onCmd(installCmd)
		}
		var outBuf bytes.Buffer
		installCmd.Stdout = io.MultiWriter(logWriter, &outBuf)
		installCmd.Stderr = io.MultiWriter(logWriter, &outBuf)
		
		if err := installCmd.Run(); err != nil {
			fmt.Fprintf(logWriter, "[Node Agent] ❌ Ошибка установки %s: %v\n", pkgName, err)
		} else {
			fmt.Fprintf(logWriter, "[Node Agent] ✅ Пакет %s успешно установлен!\n", pkgName)
		}
	}
	return nil
}

func ensureNodeDeps(ctx context.Context, code, workDir string, logWriter io.Writer, onCmd CommandReporter) error {
	npmBin := findExecutable("npm", "yarn", "pnpm", "/usr/bin/npm", "/usr/local/bin/npm")
	if npmBin == "" {
		return nil
	}

	// Match: require('pkg'), require("pkg"), from 'pkg', from "pkg"
	reRequire := regexp.MustCompile(`(?:require\s*\(\s*['"]|from\s+['"])(@?[a-zA-Z0-9_\-\./]+)['"]`)
	matches := reRequire.FindAllStringSubmatch(code, -1)

	detected := make(map[string]bool)
	for _, m := range matches {
		if len(m) > 1 {
			importPath := m[1]
			// ignore local paths
			if strings.HasPrefix(importPath, ".") || strings.HasPrefix(importPath, "/") {
				continue
			}
			// strip node: prefix
			importPath = strings.TrimPrefix(importPath, "node:")
			parts := strings.Split(importPath, "/")
			pkg := parts[0]
			if strings.HasPrefix(pkg, "@") && len(parts) > 1 {
				pkg = parts[0] + "/" + parts[1]
			}
			if !nodeBuiltIns[pkg] {
				detected[pkg] = true
			}
		}
	}

	for pkg := range detected {
		fmt.Fprintf(logWriter, "[Node Agent] 📦 Обнаружен пакет Node.js '%s'. Проверка и установка...\n", pkg)
		cmd := exec.CommandContext(ctx, npmBin, "install", "--no-audit", "--no-fund", pkg)
		if onCmd != nil {
			onCmd(cmd)
		}
		cmd.Dir = workDir
		cmd.Stdout = logWriter
		cmd.Stderr = logWriter
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(logWriter, "[Node Agent] ❌ Ошибка npm install %s: %v\n", pkg, err)
		} else {
			fmt.Fprintf(logWriter, "[Node Agent] ✅ Node модуль %s готов к работе!\n", pkg)
		}
	}
	return nil
}

func ensureGoDeps(ctx context.Context, workDir string, logWriter io.Writer, onCmd CommandReporter) error {
	goBin := findExecutable("go", "/usr/bin/go", "/usr/local/go/bin/go")
	if goBin == "" {
		fmt.Fprintf(logWriter, "[Node Agent] go не найден в PATH!\n")
		return nil
	}

	// Run go mod tidy in script directory
	fmt.Fprintf(logWriter, "[Node Agent] 📦 Обновление модулей Golang (go mod tidy)...\n")
	cmd := exec.CommandContext(ctx, goBin, "mod", "tidy")
	if onCmd != nil {
		onCmd(cmd)
	}
	cmd.Dir = workDir
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	_ = cmd.Run()
	return nil
}

// StreamCommandOutput executes command and pipes lines in real time
func StreamCommandOutput(cmd *exec.Cmd, outWriter io.Writer) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	pipeReader := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			fmt.Fprintf(outWriter, "%s\n", scanner.Text())
		}
	}

	go pipeReader(stdout)
	go pipeReader(stderr)

	return cmd.Wait()
}
