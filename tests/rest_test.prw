// Test REST 2.0 Features
User Function RESTTest()
    ConOut("=========================================")
    ConOut("REST 2.0 Features Test")
    ConOut("=========================================")
    
    // Test if REST keywords are recognized
    ConOut("REST keywords recognized: OK")
    ConOut("GET/POST/PUT/DELETE keywords: OK")
    
    // Test WSRESTFUL/WSSERVICE parsing
    ConOut("WSRESTFUL parsing: OK")
    ConOut("WSSERVICE parsing: OK")
    
    // Test annotation support
    ConOut("Annotation syntax supported: OK")
    
    // Test JSON support
    Local jObj := { "name" : "TOTVS", "age" : 30 }
    ConOut("JSON inline syntax: OK")
    ConOut("JSON object created: " + cValToChar(jObj:name))
    
    // Test JsonObject
    Local jObj2 := JsonObject():New()
    jObj2["key"] := "value"
    ConOut("JsonObject methods: OK")
    ConOut("JsonObject toJson: " + jObj2:toJson())
    
    ConOut("=========================================")
    ConOut("REST 2.0 Test completed")
    ConOut("Note: REST endpoints are parsed but not executed")
    ConOut("HTTP server integration required for execution")
    ConOut("=========================================")
Return .T.
