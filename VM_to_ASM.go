package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var fileName string

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <directory>")
		os.Exit(1)
	}

	dir := os.Args[1]
	dirName := filepath.Base(dir)
	asmFilePath := filepath.Join(dir, dirName+".asm")

	// Create or truncate the .asm file.
	asmFile, err := os.Create(asmFilePath) // If this line causes an error, 'asmFile' and 'err' might be declared elsewhere.
	if err != nil {
		fmt.Printf("Error creating ASM file: %s\n", err)
		os.Exit(1)
	}

	writer := bufio.NewWriter(asmFile)

	if _, err := writer.WriteString("@256\nD=A\n@SP\nM=D\n"); err != nil {
		fmt.Printf("Error initializing stack %s\n", err)
		os.Exit(1)
	}
	defer asmFile.Close()
	writer.Flush() // Ensure buffer is flushed at the end

	files, err := os.ReadDir(dir) // If 'err' was declared above, use '=' instead of ':=' here.
	if err != nil {
		fmt.Printf("Error reading directory: %s\n", err)
		os.Exit(1)
	}

	hasSysVM := checkForSysVM(files)
	// Process Sys.vm first if it exists
	if hasSysVM {
		sysVMPath := filepath.Join(dir, "Sys.vm")
		err := translateVMFileToASM(sysVMPath, asmFile)
		if err != nil {
			fmt.Printf("Error translating %s: %s\n", sysVMPath, err)
		} else {
			fmt.Printf("Translated %s\n", sysVMPath)
		}
	}

	for _, entry := range files {
		if !entry.IsDir() {
			fileName := entry.Name()
			if strings.HasSuffix(fileName, ".vm") && fileName != "Sys.vm" {
				vmFilePath := filepath.Join(dir, fileName)
				err := translateVMFileToASM(vmFilePath, asmFile) // If 'err' was declared above, use '=' instead of ':=' here.
				if err != nil {
					fmt.Printf("Error translating %s: %s\n", vmFilePath, err)
					continue
				}
			}
		}
	}

	fmt.Printf("Translation complete: %s\n", asmFilePath)
}

// Function to check if Sys.vm exists in the directory
func checkForSysVM(files []os.DirEntry) bool {
	for _, entry := range files {
		if !entry.IsDir() && entry.Name() == "Sys.vm" {
			return true
		}
	}
	return false
}

func translateVMFileToASM(vmFilePath string, asmFile *os.File) error {
	vmFile, err := os.Open(vmFilePath)
	if err != nil {
		return err
	}
	defer vmFile.Close()

	scanner := bufio.NewScanner(vmFile)
	writer := bufio.NewWriter(asmFile)

	for scanner.Scan() {
		vmCommand := scanner.Text()
		vmCommand = strings.TrimSpace(vmCommand)
		if vmCommand == "" || strings.HasPrefix(vmCommand, "//") {
			continue // Skip empty lines and comments
		}

		// Assuming vmCommandToAsm is a function that translates a single VM command to Hack assembly
		// and fileName is a variable holding the name of the VM file without the ".vm" extension.
		asm, err := vmCommandToAsm(vmCommand)
		if err != nil {
			return err
		}

		if _, err := writer.WriteString(asm); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return writer.Flush()
}

var compLabelCounter int

// Define callCounter at the global scope or within a struct that maintains the state of the translation.
var callCounter int = 0
var currentFunctionName string

func vmCommandToAsm(cmd string) (string, error) {
	var asm string
	var err error // Declare the error variable here to use it in the entire function scope
	// Trim any inline comments and surrounding whitespace.
	cmd = strings.TrimSpace(strings.SplitN(cmd, "//", 2)[0])

	// Split the command into parts using whitespace as the separator.
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command line")
	}
	switch parts[0] {
	case "add", "sub", "neg", "and", "or", "not":
		asm = translateArithmeticCommand(parts[0])
	case "eq", "lt", "gt":
		asm = translateComparisonCommand(parts[0], compLabelCounter)
		compLabelCounter++
	case "push", "pop":
		if len(parts) != 3 {
			return "", fmt.Errorf("invalid %s command: %s", parts[0], cmd)
		}
		segment := parts[1]
		index := parts[2]
		if parts[0] == "push" {
			asm, err = translatePushCommand(segment, index, fileName)
		} else {
			asm, err = translatePopCommand(segment, index, fileName)
		}
		if err != nil {
			return "", err
		}
	case "function", "call":
		if len(parts) != 3 {
			return "", fmt.Errorf("invalid %s command: %s", parts[0], cmd)
		}
		functionName := parts[1]
		numLocalsOrArgs, err := strconv.Atoi(parts[2])
		if err != nil {
			return "", fmt.Errorf("invalid number for %s command: %s", parts[0], parts[2])
		}
		if parts[0] == "function" {
			asm, err = translateFunctionCommand(functionName, numLocalsOrArgs)
		} else {
			asm, err = translateCallCommand(functionName, numLocalsOrArgs)
		}
		if err != nil {
			return "", err
		}
	case "if-goto":
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid if-goto command: %s", cmd)
		}
		label := parts[1]
		asm = translateIfGotoCommand(label)
	case "goto":
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid goto command: %s", cmd)
		}
		label := parts[1]
		asm = translateGotoCommand(label)
	case "label":
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid label command: %s", cmd)
		}
		label := parts[1]
		asm = translateLabelCommand(label)
	case "return":
		if len(parts) != 1 {
			return "", fmt.Errorf("invalid return command: %s", cmd)
		}
		asm = translateReturnCommand()
	default:
		return "", fmt.Errorf("unsupported command: %s", cmd)
	}
	return asm, nil
}
func translateReturnCommand() string {
	// Assembly code to implement the VM return command.
	return "@LCL\nD=M\n@5\nA=D-A\nD=M\n@R13\nM=D\n" +
		"@SP\nA=M-1\nD=M\n@ARG\nA=M\nM=D\n" +
		"D=A+1\n@SP\nM=D\n" +
		"@LCL\nAM=M-1\nD=M\n@THAT\nM=D\n" +
		"@LCL\nAM=M-1\nD=M\n@THIS\nM=D\n" +
		"@LCL\nAM=M-1\nD=M\n@ARG\nM=D\n" +
		"@LCL\nA=M-1\nD=M\n@LCL\nM=D\n" +
		"@R13\nA=M\n0;JMP\n"

}
func translateGotoCommand(label string) string {
	// Example: goto IF_FALSE
	// @IF_FALSE
	// 0;JMP
	return fmt.Sprintf(
		"@%s\n"+
			"0;JMP\n", label)
}

func translateIfGotoCommand(label string) string {
	// Example: if-goto IF_TRUE
	// @SP
	// AM=M-1
	// D=M
	// @IF_TRUE
	// D;JNE
	return fmt.Sprintf(
		"@SP\n"+
			"AM=M-1\n"+
			"D=M\n"+
			"@%s\n"+
			"D;JNE\n", label)
}

func translateLabelCommand(label string) string {
	// The label is scoped to the current function name to ensure uniqueness within the function's scope.
	return fmt.Sprintf("(%s$%s)\n", currentFunctionName, label)
}

func translateCallCommand(functionName string, numArgs int) (string, error) {
	// The implementation of the call command is more complex and involves
	// pushing the return address and the current function's state onto the stack,
	// then repositioning the ARG and LCL pointers for the callee.
	// This is a placeholder for the actual implementation.
	// A unique return label is generated using the current function's name and a counter.
	returnLabel := fmt.Sprintf("%s$ret.%d", functionName, callCounter)
	callCounter++

	// Push the return address onto the stack.
	asm := fmt.Sprintf(
		"@%s\n"+
			"D=A\n"+
			"@SP\n"+
			"A=M\n"+
			"M=D\n"+
			"@SP\n"+
			"M=M+1\n", returnLabel)

	// Push LCL, ARG, THIS, and THAT onto the stack.
	for _, segment := range []string{"LCL", "ARG", "THIS", "THAT"} {
		asm += fmt.Sprintf(
			"@%s\n"+
				"D=M\n"+
				"@SP\n"+
				"A=M\n"+
				"M=D\n"+
				"@SP\n"+
				"M=M+1\n", segment)
	}

	// Reposition ARG (ARG = SP - numArgs - 5).
	asm += fmt.Sprintf(
		"@SP\n"+
			"D=M\n"+
			"@%d\n"+
			"D=D-A\n"+
			"@5\n"+
			"D=D-A\n"+
			"@ARG\n"+
			"M=D\n", numArgs)

	// Reposition LCL (LCL = SP).
	asm += "@SP\nD=M\n@LCL\nM=D\n"

	// Transfer control to the called function.
	asm += fmt.Sprintf("@%s\n0;JMP\n", functionName)

	// Declare a label for the return address.
	asm += fmt.Sprintf("(%s)\n", returnLabel)

	return asm, nil
}

func translateFunctionCommand(functionName string, numLocals int) (string, error) {
	var asm strings.Builder

	// Label for the function entry.
	asm.WriteString(fmt.Sprintf("(%s)\n", functionName))

	// Initialize local variables to 0.
	for i := 0; i < numLocals; i++ {
		asm.WriteString("@SP\nA=M\nM=0\n") // Set the value at the top of the stack to 0.
		asm.WriteString("@SP\nM=M+1\n")    // Increment the stack pointer.
	}

	return asm.String(), nil
}

func translatePushCommand(segment, index, fileName string) (string, error) {
	var asm string
	switch segment {
	case "constant":
		asm = fmt.Sprintf("@%s\nD=A\n@SP\nA=M\nM=D\n@SP\nM=M+1\n", index)
	case "local":
		asm = pushSegmentTemplate("LCL", index)
	case "argument":
		asm = pushSegmentTemplate("ARG", index)
	case "this":
		asm = pushSegmentTemplate("THIS", index)
	case "that":
		asm = pushSegmentTemplate("THAT", index)
	case "static":
		asm = fmt.Sprintf("@%s.%s\nD=M\n@SP\nA=M\nM=D\n@SP\nM=M+1\n", fileName, index)
	case "temp":
		offset, err := strconv.Atoi(index)
		if err != nil {
			return "", fmt.Errorf("invalid index for push temp command: %s", index)
		}
		asm = fmt.Sprintf("@R%d\nD=M\n@SP\nA=M\nM=D\n@SP\nM=M+1\n", 5+offset)
	case "pointer":
		if index == "0" {
			asm = "@THIS\nD=M\n"
		} else if index == "1" {
			asm = "@THAT\nD=M\n"
		} else {
			return "", fmt.Errorf("invalid index for push pointer command: %s", index)
		}
		asm += "@SP\nA=M\nM=D\n@SP\nM=M+1\n"
	default:
		return "", fmt.Errorf("unsupported push command: push %s %s", segment, index)
	}
	return asm, nil
}

func pushSegmentTemplate(segment, index string) string {
	return fmt.Sprintf(
		"@%s\nD=M\n@%s\nA=D+A\nD=M\n@SP\nA=M\nM=D\n@SP\nM=M+1\n",
		segment, index,
	)
}

func translateArithmeticCommand(cmd string) string {
	var asm string
	switch cmd {
	case "add":
		asm = "@SP\nAM=M-1\nD=M\nA=A-1\nM=D+M\n"
	case "sub":
		asm = "@SP\nAM=M-1\nD=M\nA=A-1\nM=M-D\n"
	case "neg":
		asm = "@SP\nA=M-1\nM=-M\n"
	case "and":
		asm = "@SP\nAM=M-1\nD=M\nA=A-1\nM=D&M\n"
	case "or":
		asm = "@SP\nAM=M-1\nD=M\nA=A-1\nM=D|M\n"
	case "not":
		asm = "@SP\nA=M-1\nM=!M\n"
	}
	return asm
}

func translateComparisonCommand(cmd string, labelCounter int) string {
	jumpInstruction := map[string]string{
		"eq": "JEQ",
		"lt": "JLT",
		"gt": "JGT",
	}[cmd]

	trueLabel := fmt.Sprintf("%s_TRUE_%d", strings.ToUpper(cmd), labelCounter)
	endLabel := fmt.Sprintf("%s_END_%d", strings.ToUpper(cmd), labelCounter)

	return fmt.Sprintf(
		"@SP\nAM=M-1\nD=M\nA=A-1\nD=M-D\n@%s\nD;%s\n"+
			"@SP\nA=M-1\nM=0\n@%s\n0;JMP\n(%s)\n@SP\nA=M-1\nM=-1\n(%s)\n",
		trueLabel, jumpInstruction, endLabel, trueLabel, endLabel)
}

func translatePopCommand(segment, index, fileName string) (string, error) {
	var baseAddr string
	var asm string

	switch segment {
	case "local":
		baseAddr = "LCL"
	case "argument":
		baseAddr = "ARG"
	case "this":
		baseAddr = "THIS"
	case "that":
		baseAddr = "THAT"
	case "temp":
		tempIndex, err := strconv.Atoi(index)
		if err != nil {
			return "", fmt.Errorf("invalid index for pop temp command: %s", index)
		}
		return fmt.Sprintf(
			"@%d\nD=A\n@%d\nD=D+A\n@R13\nM=D\n@SP\nAM=M-1\nD=M\n@R13\nA=M\nM=D\n",
			5, tempIndex, // Temp segment starts at RAM address 5
		), nil
	case "pointer":
		if index == "0" {
			baseAddr = "THIS"
		} else if index == "1" {
			baseAddr = "THAT"
		} else {
			return "", fmt.Errorf("invalid index for pop pointer command: %s", index)
		}
		return fmt.Sprintf(
			"@SP\nAM=M-1\nD=M\n@%s\nM=D\n",
			baseAddr,
		), nil
	case "static":
		staticIndex, err := strconv.Atoi(index)
		if err != nil {
			return "", fmt.Errorf("invalid index for pop static command: %s", index)
		}
		return fmt.Sprintf(
			"@SP\nAM=M-1\nD=M\n@%s.%d\nM=D\n",
			fileName, staticIndex, // Static segment is file-scoped
		), nil
	default:
		return "", fmt.Errorf("unsupported segment for pop command: %s", segment)
	}

	// For local, argument, this, that
	asm = fmt.Sprintf(
		"@%s\nD=M\n@%s\nD=D+A\n@R13\nM=D\n@SP\nAM=M-1\nD=M\n@R13\nA=M\nM=D\n",
		baseAddr, index,
	)
	return asm, nil
}
