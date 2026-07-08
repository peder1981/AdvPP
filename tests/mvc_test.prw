// MVC Test - Simple Form with Model and View
User Function MVCTest()
    Local oModel
    Local oView
    Local oBrowse
    
    ConOut("=========================================")
    ConOut("MVC Test - FormModel and FormView")
    ConOut("=========================================")
    
    // Create Model using native function
    oModel := FWFormModel("CustomerModel")
    If ValType(oModel) == "O"
        ConOut("Model created successfully")
    Else
        ConOut("Error: Model not created")
        ConOut("Type returned: " + ValType(oModel))
        Return .F.
    EndIf
    
    // Create View using native function
    oView := FWFormView("CustomerView", oModel)
    If ValType(oView) == "O"
        ConOut("View created successfully")
    Else
        ConOut("Error: View not created")
        ConOut("Type returned: " + ValType(oView))
        Return .F.
    EndIf
    
    // Create Browse using native function
    oBrowse := FWFormBrowse("CustomerBrowse", oModel)
    If ValType(oBrowse) == "O"
        ConOut("Browse created successfully")
    Else
        ConOut("Error: Browse not created")
        ConOut("Type returned: " + ValType(oBrowse))
        Return .F.
    EndIf
    
    ConOut("=========================================")
    ConOut("MVC Test completed successfully!")
    ConOut("=========================================")
Return .T.
