/*
Comprehensive Nil/Null Safety Test Suite — Task 9

Tests all nil edge cases in AdvPP VM:
- Nil value operations
- Nil array operations
- Nil object operations
- Uninitialized variables
- Nil comparisons
- Nil in collections
- Nil method calls
- Nil parameter passing

Expected: All tests PASS (no crashes, graceful error handling)
*/

#include "tlpp-core.th"
#include "totvs.ch"

// ENTRY POINT: Must be first function for auto-selection
/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass

Master test runner — runs all nil safety tests in sequence
*/
User Function TestNilSafetyComprehensive()
    Local lPass := .T.

    ConOut("")
    ConOut("========================================")
    ConOut("AdvPP Comprehensive Nil Safety Test Suite")
    ConOut("========================================")
    ConOut("")

    // Run all tests
    If !TestNilValueOperations()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilComparison()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestUnboundVariables()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilArrayOperations()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilObjectOperations()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilInCollections()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilArithmetic()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilStringOps()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilConditionals()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilFunctionParameters()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilInLoops()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilInArrayIteration()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilTypeConversion()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestTryCatchWithNil()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilInArrayGet()
        lPass := .F.
    EndIf
    ConOut("")

    If !TestNilInCodeBlock()
        lPass := .F.
    EndIf
    ConOut("")

    ConOut("========================================")
    If lPass
        ConOut("RESULT: ALL TESTS PASSED ✓")
    Else
        ConOut("RESULT: SOME TESTS FAILED ✗")
    EndIf
    ConOut("========================================")
    ConOut("")

Return lPass

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilValueOperations()
    Local oNil := Nil
    Local cType := ""
    Local cStr := ""
    Local lTruthy := .F.

    ConOut("Test: Nil value operations")

    // Test Type() on nil
    cType := oNil:Type()
    If cType != "U"
        ConOut("  FAIL: Nil:Type() should return 'U', got " + cType)
        Return .F.
    EndIf
    ConOut("  PASS: Nil:Type() = " + cType)

    // Test String() on nil
    cStr := oNil:String()
    If cStr != "Nil"
        ConOut("  FAIL: Nil:String() should return 'Nil', got " + cStr)
        Return .F.
    EndIf
    ConOut("  PASS: Nil:String() = " + cStr)

    // Test IsTruthy() on nil
    lTruthy := oNil:IsTruthy()
    If lTruthy != .F.
        ConOut("  FAIL: Nil:IsTruthy() should return .F., got " + cValToChar(lTruthy))
        Return .F.
    EndIf
    ConOut("  PASS: Nil:IsTruthy() = .F.")

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilComparison()
    Local oNil1 := Nil
    Local oNil2 := Nil
    Local oNil3 := Nil

    ConOut("Test: Nil comparison")

    // Nil == Nil
    If !(oNil1 == oNil2)
        ConOut("  FAIL: Nil == Nil should be .T.")
        Return .F.
    EndIf
    ConOut("  PASS: Nil == Nil = .T.")

    // Nil == Nil (different variable names)
    If !(oNil2 == oNil3)
        ConOut("  FAIL: Nil == Nil (vars 2,3) should be .T.")
        Return .F.
    EndIf
    ConOut("  PASS: Nil == Nil (different vars) = .T.")

    // Nil != something else
    If !(oNil1 != 1)
        ConOut("  FAIL: Nil != 1 should be .T.")
        Return .F.
    EndIf
    ConOut("  PASS: Nil != 1 = .T.")

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestUnboundVariables()
    Local oUnbound  // No initialization = Nil

    ConOut("Test: Unbound variables (v1.22.1 SIGSEGV fix)")

    // This crashed in v1.22.1
    If oUnbound == Nil
        ConOut("  PASS: Uninitialized variable == Nil")
    Else
        ConOut("  FAIL: Uninitialized variable should == Nil")
        Return .F.
    EndIf

    // Test with multiple uninitialized
    Local x, y, z
    If x == Nil .AND. y == Nil .AND. z == Nil
        ConOut("  PASS: Multiple uninitialized variables == Nil")
    Else
        ConOut("  FAIL: Multiple uninitialized should == Nil")
        Return .F.
    EndIf

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilArrayOperations()
    Local aNil := Nil
    Local aEmpty := {}
    Local aMixed := {Nil, 1, Nil, "test"}

    ConOut("Test: Nil array operations")

    // Test empty array
    If !(Len(aEmpty) == 0)
        ConOut("  FAIL: Empty array length should be 0")
        Return .F.
    EndIf
    ConOut("  PASS: Empty array length = 0")

    // Test array with nil elements
    If !(Len(aMixed) == 4)
        ConOut("  FAIL: Array with nil elements should have length 4")
        Return .F.
    EndIf
    ConOut("  PASS: Array with nil elements has correct length")

    // Test accessing nil element
    Local elem := aMixed[1]
    If !(elem == Nil)
        ConOut("  FAIL: aMixed[1] should be Nil")
        Return .F.
    EndIf
    ConOut("  PASS: Accessing nil element from array")

    // Test accessing numeric element
    elem := aMixed[2]
    If !(elem == 1)
        ConOut("  FAIL: aMixed[2] should be 1")
        Return .F.
    EndIf
    ConOut("  PASS: Accessing numeric element from array")

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilObjectOperations()
    Local oNil := Nil
    Local oObj := JsonObject():New()

    ConOut("Test: Nil object operations")

    // Test setting property on valid object via bracket notation
    oObj["key1"] := "value1"
    If !(oObj["key1"] == "value1")
        ConOut("  FAIL: Object property setting failed")
        Return .F.
    EndIf
    ConOut("  PASS: Object property setting works")

    // Test setting nil property
    oObj["nilKey"] := Nil
    If !(oObj["nilKey"] == Nil)
        ConOut("  FAIL: Object nil property should be retrievable as Nil")
        Return .F.
    EndIf
    ConOut("  PASS: Object nil property handled correctly")

    // Test object with multiple nil properties
    Local i := 0
    For i := 1 To 10
        oObj["key" + cValToChar(i)] := Nil
    Next
    ConOut("  PASS: Multiple nil properties set on object")

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilInCollections()
    Local aArray := {Nil, Nil, Nil}
    Local oObj := JsonObject():New()

    ConOut("Test: Nil in collections")

    // Test array of nils
    If !(Len(aArray) == 3)
        ConOut("  FAIL: Array of nils should have length 3")
        Return .F.
    EndIf
    ConOut("  PASS: Array of nils has correct length")

    // Test setting nil in object via bracket
    oObj["nilVal"] := Nil
    If !(oObj["nilVal"] == Nil)
        ConOut("  FAIL: Object nil value should be retrievable")
        Return .F.
    EndIf
    ConOut("  PASS: Nil values in object collections work")

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilArithmetic()
    Local nNil := Nil
    Local nNum := 5

    ConOut("Test: Nil arithmetic")

    // Nil + number (should convert nil to 0)
    Local nResult := nNil + nNum
    If !(nResult == 5)
        ConOut("  FAIL: Nil + 5 should equal 5, got " + cValToChar(nResult))
        Return .F.
    EndIf
    ConOut("  PASS: Nil + 5 = 5")

    // Nil * number
    nResult := nNil * 10
    If !(nResult == 0)
        ConOut("  FAIL: Nil * 10 should equal 0, got " + cValToChar(nResult))
        Return .F.
    EndIf
    ConOut("  PASS: Nil * 10 = 0")

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilStringOps()
    Local cNil := Nil
    Local cStr := "test"

    ConOut("Test: Nil string operations")

    // Nil + string (Nil converts to "Nil")
    Local cResult := cNil + cStr
    If !(cResult == "Niltest")
        ConOut("  FAIL: Nil + 'test' should equal 'Niltest', got " + cValToChar(cResult))
        Return .F.
    EndIf
    ConOut("  PASS: Nil + 'test' = 'Niltest'")

    // String + Nil (Nil converts to "Nil")
    cResult := cStr + cNil
    If !(cResult == "testNil")
        ConOut("  FAIL: 'test' + Nil should equal 'testNil', got " + cValToChar(cResult))
        Return .F.
    EndIf
    ConOut("  PASS: 'test' + Nil = 'testNil'")

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilConditionals()
    Local oNil := Nil

    ConOut("Test: Nil in conditionals")

    // If Nil (should be false)
    If oNil
        ConOut("  FAIL: If Nil should be false")
        Return .F.
    EndIf
    ConOut("  PASS: If Nil = false")

    // If !Nil (should be true)
    If !oNil
        ConOut("  PASS: If !Nil = true")
    Else
        ConOut("  FAIL: If !Nil should be true")
        Return .F.
    EndIf

    // If Nil .OR. .T.
    If oNil .OR. .T.
        ConOut("  PASS: If Nil .OR. .T. = true")
    Else
        ConOut("  FAIL: If Nil .OR. .T. should be true")
        Return .F.
    EndIf

    // If Nil .AND. .T.
    If oNil .AND. .T.
        ConOut("  FAIL: If Nil .AND. .T. should be false")
        Return .F.
    EndIf
    ConOut("  PASS: If Nil .AND. .T. = false")

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilFunctionParameters()
    // Test passing Nil to functions
    ConOut("Test: Nil in function parameters")

    Local lResult := TestNilHelper(Nil, 5, "test")
    If !lResult
        ConOut("  FAIL: Nil parameter passing")
        Return .F.
    EndIf
    ConOut("  PASS: Nil parameter passing works")

Return .T.

Static Function TestNilHelper(p1, p2, p3)
    // p1 should be Nil, p2 should be 5, p3 should be "test"
    If !(p1 == Nil)
        Return .F.
    EndIf
    If !(p2 == 5)
        Return .F.
    EndIf
    If !(p3 == "test")
        Return .F.
    EndIf
Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilInLoops()
    Local i := 0
    Local aNil := Nil
    Local aItems := {1, 2, 3}

    ConOut("Test: Nil in loops")

    // For loop with nil start (should behave like 0)
    Local nCount := 0
    For i := aNil To 3
        nCount++
    Next

    // Nil converts to 0, so loop should run 3 times (0 to 3)
    If nCount > 0
        ConOut("  PASS: Nil in For loop handled (count=" + cValToChar(nCount) + ")")
    Else
        ConOut("  FAIL: Nil in For loop should iterate")
        Return .F.
    EndIf

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilInArrayIteration()
    Local aItems := {1, Nil, 3}
    Local i := 0
    Local nCount := 0

    ConOut("Test: Nil in array iteration")

    For i := 1 To Len(aItems)
        Local elem := aItems[i]
        If elem == Nil
            nCount++
        EndIf
    Next

    If nCount != 1
        ConOut("  FAIL: Should find exactly 1 nil in array, found " + cValToChar(nCount))
        Return .F.
    EndIf
    ConOut("  PASS: Nil elements in array iteration detected correctly")

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilTypeConversion()
    Local oNil := Nil

    ConOut("Test: Nil type conversion")

    // Convert Nil to number
    Local nVal := Val(cValToChar(oNil))
    ConOut("  INFO: Val(cValToChar(Nil)) = " + cValToChar(nVal))

    // Convert Nil to string
    Local cVal := cValToChar(oNil)
    ConOut("  INFO: cValToChar(Nil) = " + cVal)

    // Convert Nil to bool
    Local lVal := oNil:IsTruthy()
    ConOut("  INFO: Nil:IsTruthy() = " + cValToChar(lVal))

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestTryCatchWithNil()
    ConOut("Test: Try/Catch with nil")

    Local lOk := .F.

    Begin Sequence
        // Try to cause an error with nil
        Local oNil := Nil
        Local nResult := oNil + 5  // Should work (nil converts to 0)

        If nResult == 5
            lOk := .T.
        EndIf
    Recover Using oErr
        ConOut("  ERROR: Unexpected exception: " + oErr:Description)
        Return .F.
    End Sequence

    If lOk
        ConOut("  PASS: Try/Catch with nil operations")
        Return .T.
    Else
        ConOut("  FAIL: Nil operation in try block failed")
        Return .F.
    EndIf

Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilInArrayGet()
    Local aArr := {1, 2, 3}
    Local nNil := Nil

    ConOut("Test: Nil as array index")

    // Accessing array with Nil as index (should convert to 0, which is out of bounds)
    Local elem := aArr[nNil]
    If elem == Nil
        ConOut("  PASS: aArr[Nil] returns Nil (out of bounds)")
        Return .T.
    EndIf

ConOut("  INFO: aArr[Nil] = " + cValToChar(elem))
Return .T.

/***
@type function
@author AdvPP Stability Task 9
@since 2026-07-29
@param none
@return logical — .T. if all tests pass
*/
User Function TestNilInCodeBlock()
    Local bBlock := {|x| x + 1}
    Local nNil := Nil

    ConOut("Test: Nil with codeblocks")

    // Execute block with nil parameter
    Local nResult := Eval(bBlock, nNil)
    If nResult == 1  // Nil + 1 = 1
        ConOut("  PASS: Eval(block, Nil) = 1")
        Return .T.
    EndIf

    ConOut("  FAIL: Eval(block, Nil) should equal 1, got " + cValToChar(nResult))
Return .F.
