const fields=['species','sex','weight','individualNumber','hunterName','captureLocation','captureType','estimatedAge'];
/** Bounded conversational context only. Never send the full form/catalog or device configuration. */
export function interpretationInput(text:string,values:Record<string,unknown>,options:any,history:Array<{role:string;text:string}>){
 const selected:Record<string,string>={};for(const key of fields){const value=values[key];if(typeof value==='string'||typeof value==='number')selected[key]=String(value).slice(0,100);}
 const choices:Record<string,Array<{value:string;label:string}>>={};
 for(const key of ['hunterName','captureLocation']){const list=options?.[key];if(Array.isArray(list))choices[key]=list.filter(v=>v&&typeof v.value==='string'&&typeof v.label==='string').slice(0,10).map(v=>({value:v.value.slice(0,40),label:v.label.slice(0,40)}));}
 const result={text:text.slice(0,1000),values:selected,options:choices,history:history.slice(-6).map(v=>({role:v.role,text:v.text.slice(0,150)}))};
 while(new TextEncoder().encode(JSON.stringify(result)).length>12000){if(result.history.length)result.history.shift();else {const list=Object.values(choices).find(v=>v.length);if(!list)break;list.pop();}}
 return result;
}
