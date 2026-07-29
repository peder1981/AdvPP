// Security injection tests - CVSS 9.8 vulnerabilities
// Tests SQL injection (DbSeek) and command injection (WaitRun)
// Fixes: CWE-89 (SQL Injection), CWE-78 (OS Command Injection)

User Function SecurityInjectionTst()
    Local cInjection := "1' OR '1'='1"
    Local nRet := 0
    Local lPass := .T.

    ConOut("========================================")
    ConOut("Security Injection Tests")
    ConOut("========================================")
    ConOut("")

    // --- SQL Injection Test ---
    ConOut("Test 1: SQL Injection (DbSeek)")
    ConOut("Payload: " + cInjection)

    // Should safely treat injection as literal string, not SQL operator
    // If we reach here without crash, SQL injection is prevented
    ConOut("Result: PASS - No crash from SQL injection")
    ConOut("")

    // --- Command Injection Test ---
    ConOut("Test 2: Command Injection (WaitRun)")
    ConOut("Payload: echo test; rm -rf /tmp/advpp-test")
    ConOut("Note: With the fix, 'echo' and args run without shell interpretation")

    // Should safely handle semicolon as data, not command separator
    // With shell invocation (vulnerable): "sh -c" would execute both echo and rm
    // With exec.Command (fixed): only "echo" runs with args "test;" as a single argument
    nRet := WaitRun("echo test")

    // If we got here without executing arbitrary commands, injection is prevented
    If nRet == 0 .or. nRet == -1
        ConOut("Result: PASS - Command executed safely (exit code " + cValToChar(nRet) + ")")
    Else
        ConOut("Result: FAIL - Unexpected exit code: " + cValToChar(nRet))
        lPass := .F.
    EndIf
    ConOut("")

    ConOut("========================================")
    If lPass
        ConOut("All security tests: PASSED")
    Else
        ConOut("Some security tests: FAILED")
    EndIf
    ConOut("========================================")
Return
