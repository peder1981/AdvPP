// Geometria espacial (S6c): vetores 2D/3D.
User Function GeoTst()
    Local nFail := 0
    Local aC := {}
    Local aR := {}
    Local nPi := 3.14159265358979

    // cross({1,0,0},{0,1,0}) = {0,0,1}
    aC := VecCross({1,0,0}, {0,1,0})
    If aC[1] != 0 .Or. aC[2] != 0 .Or. Abs(aC[3]-1) > 0.0001
        ConOut("FALHA: VecCross")
        nFail++
    EndIf

    // dot ortogonais = 0
    If VecDot({1,0}, {0,1}) != 0
        ConOut("FALHA: VecDot")
        nFail++
    EndIf

    // norm({3,4}) = 5
    If Abs(VecNorm({3,4}) - 5) > 0.0001
        ConOut("FALHA: VecNorm")
        nFail++
    EndIf

    // dist({0,0},{3,4}) = 5
    If Abs(VecDist({0,0}, {3,4}) - 5) > 0.0001
        ConOut("FALHA: VecDist")
        nFail++
    EndIf

    // angle ortogonais = pi/2
    If Abs(VecAngle({1,0}, {0,1}) - nPi/2) > 0.001
        ConOut("FALHA: VecAngle")
        nFail++
    EndIf

    // normalize({3,4}) -> magnitude 1
    aR := VecNormalize({3,4})
    If Abs(VecNorm(aR) - 1) > 0.0001
        ConOut("FALHA: VecNormalize")
        nFail++
    EndIf

    // RotateVec2({1,0}, pi/2) ~ {0,1}
    aR := RotateVec2({1,0}, nPi/2)
    If Abs(aR[1]) > 0.001 .Or. Abs(aR[2]-1) > 0.001
        ConOut("FALHA: RotateVec2 (" + Str(aR[1],8,4) + "," + Str(aR[2],8,4) + ")")
        nFail++
    EndIf

    // RotateVec3 em torno de z: {1,0,0} -> {0,1,0}
    aR := RotateVec3({1,0,0}, "z", nPi/2)
    If Abs(aR[1]) > 0.001 .Or. Abs(aR[2]-1) > 0.001
        ConOut("FALHA: RotateVec3")
        nFail++
    EndIf

    If nFail == 0
        ConOut("OK: geometria espacial verificada.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,2))
    EndIf
Return
