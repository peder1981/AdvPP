# AdvPP - AdvPL/TLPP Compiler in Go

A fully functional compiler and interpreter for the AdvPL and TLPP programming languages, built in Go.

## Features

- **Lexer**: Complete tokenizer for AdvPL/TLPP syntax including keywords, operators, code blocks, and preprocessor directives
- **Preprocessor**: Handles `#include`, `#define`, `#ifdef`/`#ifndef`/`#else`/`#endif`, `#xCommand`, `#xTranslate`
- **Parser**: Full recursive descent parser producing an AST
- **Compiler**: Generates optimized bytecode with 88 opcodes
- **Bytecode Serialization**: Save compiled bytecode to disk for later execution
- **Standalone Executables**: Build self-contained executables with embedded bytecode using go:embed
- **Virtual Machine**: Complete VM with all opcodes implemented
- **Runtime**: Built-in functions (ConOut, MsgInfo, AllTrim, Str, Val, aAdd, aScan, Len, etc.)
- **GUI IDE**: Graphical Development Environment using Fyne with code editor, file browser, and integrated compiler
- **UI Framework**: Graphical applications using Fyne (dialogs, forms, grids, buttons, menus)
- **Database**: Workarea-based database operations (DbSelectArea, DbSeek, DbSkip, RecLock, etc.)
- **Classes**: Full class system with Data/Method/Constructor, inheritance via `from`
- **Code Blocks**: Executable code blocks `{|| ... }`
- **MVC**: FWFormModel, FWFormView, FWFormBrowse support

## Building

```bash
# Build command-line compiler
go build -o advplc ./cmd/advplc

# Build GUI IDE
go build -o advpp-ide ./cmd/advpp-ide
```

## Usage

### Command-Line Compiler

```bash
# Run an AdvPL/TLPP source file (compile in memory and execute)
./advplc run program.prw

# Compile source to bytecode file
./advplc compile program.prw -o program.bytecode

# Execute a compiled bytecode file
./advplc exec program.bytecode

# Build standalone executable (embeds bytecode and runtime)
./advplc build program.prw -o program

# Check syntax only
./advplc check program.prw

# Print AST structure
./advplc ast program.prw

# Print bytecode
./advplc bytecode program.prw
```

### GUI IDE

```bash
# Launch the graphical development environment
./advpp-ide
```

The GUI IDE provides:
- **Code Editor**: Multi-line text editor with support for .prw, .tlpp, and .prg files
- **File Operations**: New, Open, Save, Save As functionality
- **Project Explorer**: File browser showing current directory with source file highlighting
- **Build Integration**: Compile, Run, and Compile & Run commands
- **Output Console**: Shows compilation results and program output
- **Dialog Support**: MsgInfo, MsgStop, MsgAlert, and MsgYesNo functions display Fyne dialogs

## Language Support

### AdvPL Features
- User Function, Static Function, Function declarations
- Local, Private, Public, Static variable scopes
- If/ElseIf/Else/EndIf, For/Next, While/EndDo, Do Case/EndCase
- Begin Sequence/Recover/End Sequence error handling
- Code blocks `{|| expr }`
- Class/EndClass with Data, Method, Constructor
- Method implementation outside class block
- Alias field access `SA1->A1_NOME`
- Self-reference `::property`
- All AdvPL data types: Character, Numeric, Logical, Date, Array, Code Block, Nil, Object

### TLPP Additional Features
- Static typing with `as` keyword
- Try/Catch/EndTry error handling
- Namespace declarations
- Access modifiers (Public, Private, Protected)
- REST annotations (@Get, @Post, @Put, @Delete)
- JSON inline support
- Long identifiers (with namespace)
- Integer, Double, Decimal, Variant, Variadic types
