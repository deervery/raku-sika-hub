import type {FormDefinition} from './types/forms';
import {reviewEntries,missingValue} from './lib/voice-guidance/stationUi';
export const HEALTH_GROUP='__health_checks';
export function healthEntries(form:FormDefinition,values:Record<string,unknown>){return reviewEntries(form,values,'all').filter(([,f])=>f.options?.includes('異常なし')&&f.options?.includes('異常あり'));}
export function receivingEntries(form:FormDefinition,values:Record<string,unknown>,mode:'all'|'filled'|'missing',focus?:string[]){
 const health=healthEntries(form,values);const keys=new Set(health.map(([k])=>k));
 const entries=reviewEntries(form,values,'all',focus).filter(([k])=>mode==='all'||(mode==='missing'?missingValue(values[k]):!missingValue(values[k])));
 const visibleHealth=entries.filter(([k])=>keys.has(k));
 const rest=entries.filter(([k])=>!keys.has(k));
 if(visibleHealth.length)rest.push([HEALTH_GROUP,{label:'異常の確認',type:'text',required:false,order:60}]);
 const priority=['weight','individualNumber','species','sex',HEALTH_GROUP,'captureType','receivingDate','hunterName','captureLocation'];
 return rest.sort(([a,fa],[b,fb])=>{const ia=priority.indexOf(a),ib=priority.indexOf(b);return (ia<0?100+fa.order:ia)-(ib<0?100+fb.order:ib);});
}
export function healthSummary(form:FormDefinition,values:Record<string,unknown>){const rows=healthEntries(form,values);const abnormal=rows.filter(([k])=>values[k]==='異常あり');const missing=rows.filter(([k])=>missingValue(values[k]));return abnormal.length?`異常あり ${abnormal.length}項目`:missing.length?`未確認 ${missing.length}項目`:'異常なし';}
export function labelOptional(key:string,value:unknown){return ['species','sex','captureType'].includes(key)&&!missingValue(value);}
