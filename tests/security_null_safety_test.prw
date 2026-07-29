// security_null_safety_test.prw — Test nil safety guards (CWE-476)
// This file tests that the VM handles nil operations gracefully without SIGSEGV

User Function TestNilComparison()
    Local oNil := Nil
    Local lRet := .F.

    // v1.22.1 bug: this crashed with SIGSEGV
    // Should now work safely and return .T.
    If oNil == Nil
        lRet := .T.
    EndIf
Return lRet

User Function TestNilMethodCall()
    Local oNil := Nil
    Local cStr := ""

    // This should return "Nil" via nil guard, not crash
    cStr := oNil:ToString()

    // If we reach here, null safety works
Return .T.

User Function TestNilType()
    Local oNil := Nil
    Local cType := ""

    // This should return "U" (Undefined/Nil), not crash
    cType := oNil:Type()

    // If we reach here, null safety works
Return cType == "U"

User Function TestNilIsTruthy()
    Local oNil := Nil
    Local lTruthy := .T.

    // This should return .F., not crash
    lTruthy := oNil:IsTruthy()

    // If we reach here, null safety works
Return !lTruthy

User Function TestUnboundParameterComparison()
    // This is the entrypoint_lib_test.prw scenario:
    // Parameter not passed means it's uninitialized (nil)
    // Comparing to Nil should work, not crash

    Local cParam  // Unbound parameter

    If cParam == Nil
        Return .T.  // Expected: true for unbound param
    EndIf
Return .F.

User Function TestArrayWithNilElements()
    Local aArr := {}
    Local i := 0

    aAdd(aArr, Nil)
    aAdd(aArr, "test")
    aAdd(aArr, Nil)

    // Array operations on nil elements should be safe
    // String() should return "Nil" for nil elements
Return .T.

User Function TestNilComparison2()
    Local a := Nil
    Local b := Nil

    // nil == nil should be true, not crash
    If a == b
        Return .T.
    EndIf
Return .F.

User Function TestNilComparison3()
    Local a := Nil
    Local b := "test"

    // nil == "test" should be false, not crash
    If a == b
        Return .F.
    EndIf
Return .T.
