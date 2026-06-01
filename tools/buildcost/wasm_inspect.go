//go:build ignore
// +build ignore

// wasm_inspect: dump a WASM module section map + defined-function/export counts.
//   go run wasm_inspect.go googlesql.wasm
package main
import ("encoding/binary";"fmt";"os")
func uv(b []byte,p int)(uint64,int){v,n:=binary.Uvarint(b[p:]);return v,p+n}
func main(){
 b,_:=os.ReadFile(os.Args[1])
 if len(b)<8||string(b[:4])!="\x00asm"{fmt.Println("notwasm");return}
 p:=8;tot:=len(b)
 var impFns,defFns,exports int
 names:=map[byte]string{0:"custom",1:"type",2:"import",3:"function",4:"table",5:"memory",6:"global",7:"export",8:"start",9:"elem",10:"code",11:"data",12:"datacount"}
 for p<tot{
  id:=b[p];p++
  sz,np:=uv(b,p);p=np
  body:=p
  if id==3{ // function section: count = vec length
   c,_:=uv(b,body);defFns=int(c)
  }
  if id==2{ // import: count function imports
   c,q:=uv(b,body)
   for i:=0;i<int(c);i++{
    ml,q2:=uv(b,q);q=q2+int(ml) // module
    nl,q3:=uv(b,q);q=q3+int(nl) // name
    kind:=b[q];q++
    if kind==0{impFns++;_,q=uv(b,q)}else if kind==1{q+=1; _,q=uv(b,q); _=0 // table: elemtype + limits
     // limits: flag then min (+max)
    }else if kind==2{fl:=b[q-0];_=fl}
    // simplify: we only need impFns; break parsing precision by re-walking is risky
    _=q
   }
  }
  if id==7{c,_:=uv(b,body);exports=int(c)}
  fmt.Printf("@@SEC@@ %-9s id=%d size=%d pct=%.1f\n",names[id],id,sz,100*float64(sz)/float64(tot))
  p=body+int(sz)
 }
 fmt.Printf("@@FUNCS@@ defined=%d exports=%d total_bytes=%d\n",defFns,exports,tot)
}