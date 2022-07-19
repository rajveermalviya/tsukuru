package androidbuilder

import "runtime"

func getName(tool string) string {
	if runtime.GOOS != "windows" {
		return tool
	}

	switch tool {
	case "java":
		return "java.exe"
	case "javac":
		return "javac.exe"
	case "keytool":
		return "keytool.exe"
	case "aapt2":
		return "aapt2.exe"
	case "d8":
		return "d8.bat"
	case "zipalign":
		return "zipalign.exe"
	case "apksigner":
		return "apksigner.bat"
	case "sdkmanager":
		return "sdkmanager.bat"
	case "gradlew":
		return "gradlew.bat"
	case "adb":
		return "adb.exe"

	default:
		panic("unreachable")
	}
}
