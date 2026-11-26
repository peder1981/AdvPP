// DLL de teste: mesmas assinaturas C++ do dbapi TOTVS (extraídas do
// dbapi.so), compilada com MSVC no CI Windows para gerar exports com
// mangling MSVC real. Só serve para `advpp-dbapi-bridge --symbols`.
// Os corpos não fazem nada: não é um dbapi.
#define X __declspec(dllexport)
class X tBasicClient { public: int IsConnected(); };
class X tTOPClient {
public:
    int Connect(const char*, char*, char*, short, int);
    int Query(char*, char*);
    int SQLExec(char*, int, unsigned char);
    int GoTop(short);
    int Eof(short);
    int Bof(short);
    long long RecNo(short);
    long long RecCount(short);
    int GetColCount(short);
    char* GetFieldN(short, short);
    int GetFieldDesc(short, short, char*, char*, short*, short*, short*);
    int Skip(short, int);
    int Close(short);
    int GetError(char*, short);
    int OpenFile(char*, unsigned char, unsigned char);
    int GoBottom(short);
    int GoTo(short, long long, unsigned char);
    char* GetField(short, const char*);
    int SetFieldN(short, short, const char*);
    int SetField(short, const char*, const char*);
    int Update(short, long long);
    int Write(short);
    int DeleteR(short, long long);
    int RecallR(short, long long);
    int Commit();
    int Rollback();
    int StrCmtCtl();
    int EndCmtCtl();
    int Lock(short, long long);
    int UnLock(short, long long);
    int SetKey(short, short);
    int SetKeyC(short, const char*);
    int Seek2(short, char*, short, unsigned char);
    int Pack(short);
    int GetKeyNames(short, char*);
    int FLock(short);
    int FUnLock(short);
};
int tBasicClient::IsConnected() { return 0; }
int tTOPClient::Connect(const char*, char*, char*, short, int) { return 0; }
int tTOPClient::Query(char*, char*) { return 0; }
int tTOPClient::SQLExec(char*, int, unsigned char) { return 0; }
int tTOPClient::GoTop(short) { return 0; }
int tTOPClient::Eof(short) { return 0; }
int tTOPClient::Bof(short) { return 0; }
long long tTOPClient::RecNo(short) { return 0; }
long long tTOPClient::RecCount(short) { return 0; }
int tTOPClient::GetColCount(short) { return 0; }
char* tTOPClient::GetFieldN(short, short) { return nullptr; }
int tTOPClient::GetFieldDesc(short, short, char*, char*, short*, short*, short*) { return 0; }
int tTOPClient::Skip(short, int) { return 0; }
int tTOPClient::Close(short) { return 0; }
int tTOPClient::GetError(char*, short) { return 0; }
int tTOPClient::OpenFile(char*, unsigned char, unsigned char) { return 0; }
int tTOPClient::GoBottom(short) { return 0; }
int tTOPClient::GoTo(short, long long, unsigned char) { return 0; }
char* tTOPClient::GetField(short, const char*) { return nullptr; }
int tTOPClient::SetFieldN(short, short, const char*) { return 0; }
int tTOPClient::SetField(short, const char*, const char*) { return 0; }
int tTOPClient::Update(short, long long) { return 0; }
int tTOPClient::Write(short) { return 0; }
int tTOPClient::DeleteR(short, long long) { return 0; }
int tTOPClient::RecallR(short, long long) { return 0; }
int tTOPClient::Commit() { return 0; }
int tTOPClient::Rollback() { return 0; }
int tTOPClient::StrCmtCtl() { return 0; }
int tTOPClient::EndCmtCtl() { return 0; }
int tTOPClient::Lock(short, long long) { return 0; }
int tTOPClient::UnLock(short, long long) { return 0; }
int tTOPClient::SetKey(short, short) { return 0; }
int tTOPClient::SetKeyC(short, const char*) { return 0; }
int tTOPClient::Seek2(short, char*, short, unsigned char) { return 0; }
int tTOPClient::Pack(short) { return 0; }
int tTOPClient::GetKeyNames(short, char*) { return 0; }
int tTOPClient::FLock(short) { return 0; }
int tTOPClient::FUnLock(short) { return 0; }
extern "C" X void* v40TC_GetTopConn(int, short) { return nullptr; }
extern "C" X void v40TC_Release(void*) {}
