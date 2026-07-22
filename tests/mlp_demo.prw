// MLP float pequeno: entrada X[1,2] -> Linear(2x2)+bias -> Relu -> Linear(2x2)+bias -> Softmax -> Argmax.
// Pesos fixos; resultado conferido contra cálculo manual.
User Function MlpDemo()
    Local oX  := Tensor():FromArray({1, 2}, {1, 2})
    Local oW1 := Tensor():FromArray({1, 0, 0, 1}, {2, 2})   // identidade
    Local ob1 := Tensor():FromArray({0, -3}, {2})           // bias
    Local oW2 := Tensor():FromArray({1, 0, 0, 1}, {2, 2})   // identidade
    Local ob2 := Tensor():FromArray({0, 0}, {2})
    Local oH, oY, nPred, aY, nFail := 0

    // h = relu(X·W1 + b1) = relu([1,2] + [0,-3]) = relu([1,-1]) = [1,0]
    oH := oX:MatMul(oW1):Add(ob1):Relu()
    // y = softmax(h·W2 + b2) = softmax([1,0])
    oY := oH:MatMul(oW2):Add(ob2):Softmax(2)
    aY := oY:ToArray()
    nPred := oY:Argmax()          // maior prob -> classe 1 (offset 1-based = 1)

    If Abs(oH:ToArray()[1] - 1) > 0.001 .Or. Abs(oH:ToArray()[2] - 0) > 0.001
        ConOut("FALHA camada oculta: " + Str(oH:ToArray()[1]) + "," + Str(oH:ToArray()[2])); nFail++
    EndIf
    // softmax([1,0]) = [e/(e+1), 1/(e+1)] ~ [0.731, 0.269]
    If Abs(aY[1] - 0.7311) > 0.001
        ConOut("FALHA softmax: " + Str(aY[1])); nFail++
    EndIf
    If nPred != 1
        ConOut("FALHA argmax: " + Str(nPred)); nFail++
    EndIf

    ConOut("MLP forward: h=[" + Str(oH:ToArray()[1],3,1) + "," + Str(oH:ToArray()[2],3,1) + "]" + ;
           " y=[" + Str(aY[1],5,3) + "," + Str(aY[2],5,3) + "] pred=" + Str(nPred,1))
    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1))
    EndIf
Return
