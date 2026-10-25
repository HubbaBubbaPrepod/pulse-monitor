package main

import("encoding/json";"os")

func main(){
 var input struct{Protocol int `json:"protocol"`;Config map[string]any `json:"config"`;Input map[string]any `json:"input"`;State json.RawMessage `json:"state"`}
 if json.NewDecoder(os.Stdin).Decode(&input)!=nil {os.Exit(1)}
 if input.Config["hang"]==true {for {}}
 json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol":input.Protocol,"status":"ok","data":map[string]any{"config":input.Config,"input":input.Input,"state":input.State}})
}
