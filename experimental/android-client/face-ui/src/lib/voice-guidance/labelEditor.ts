import type {FormField} from '@/types/forms';
import {PLACEHOLDER_SCHEMA,type PlaceholderName} from '@/lib/labels/placeholderSchema';
import {LABEL_TEMPLATES,type LabelDraft,labelFields} from './stationOperations';

export const QUANTITY_UNITS=['kg','g','個','包','袋'] as const;
export const STORAGE_OPTIONS=['-18℃以下','4℃以下','10℃以下','直射日光と高温多湿を避けて保管してください'];
export function parseQuantity(raw:string){
  const match=raw.trim().match(/^(\d+(?:\.\d+)?)\s*(kg|g|個|包|袋)$/);
  return match?{amount:match[1],unit:match[2]}:null;
}
export function validQuantity(raw:string){const q=parseQuantity(raw);return !!q&&Number(q.amount)>0&&Number.isFinite(Number(q.amount))&&(['kg','g'].includes(q.unit)||Number.isInteger(Number(q.amount)));}
export function validDate(raw:string){
  if(!/^\d{4}-\d{2}-\d{2}$/.test(raw))return false;
  const [y,m,d]=raw.split('-').map(Number);const value=new Date(Date.UTC(y,m-1,d));
  return y>=1900&&y<=2199&&value.getUTCFullYear()===y&&value.getUTCMonth()===m-1&&value.getUTCDate()===d;
}
/** Explicit label utterances only. Never send label input into the receiving form parser. */
export function labelIntent(text:string):any {
  const s=text.normalize('NFKC').replace(/[。！!\s]/g,'').replace(/(?:です|に訂正|に変更)$/,'');
  let match=s.match(/^(?:内容量(?:は)?)?(\d+(?:\.\d+)?)(kg|キロ|キログラム|g|グラム|個|包|袋)$/i);
  if(match){const unit=/^(kg|キロ|キログラム)$/i.test(match[2])?'kg':/^(g|グラム)$/i.test(match[2])?'g':match[2];return {label:{data:{productQuantity:match[1]+unit}}};}
  match=s.match(/^(?:枚数(?:は)?)?(\d+)枚$/);if(match)return {label:{copies:Number(match[1])}};
  match=s.match(/^(個体番号|個体識別番号|製品名|捕獲場所|原材料名)(?:は)?(.+)$/);
  if(match){const keys:Record<string,string>={'個体番号':'individualNumber','個体識別番号':'individualNumber','製品名':'productName','捕獲場所':'captureLocation','原材料名':'productIngredient'};return {label:{data:{[keys[match[1]]]:match[2]}}};}
  match=s.match(/^(搬入日|賞味期限|消費期限|期限日)(?:は)?(\d{4})年(\d{1,2})月(\d{1,2})日$/);
  if(match){const data:Record<string,string>={[match[1]==='搬入日'?'receivingDate':'deadlineDate']:`${match[2]}-${match[3].padStart(2,'0')}-${match[4].padStart(2,'0')}`};if(['賞味期限','消費期限'].includes(match[1]))data.deadlineLabel=match[1];return {label:{data}};}
  if(['シカ','クマ','イノシシ','アライグマ'].includes(s))return {label:{data:{species:s}}};
  if(['オス','メス'].includes(s))return {label:{data:{sex:s}}};
  return {question:'ラベルの項目名と値を教えてください。内容量、個体番号、期限日、枚数を変更できます。'};
}
export function fieldForLabel(key:string,draft:LabelDraft):FormField{
  const metadata=PLACEHOLDER_SCHEMA[key as PlaceholderName];
  const options:Record<string,string[]>={template:Object.entries(LABEL_TEMPLATES).map(([v,l])=>`${v}:${l}`),species:['シカ','クマ','イノシシ','アライグマ'],sex:['オス','メス'],deadlineLabel:['賞味期限','消費期限'],storageTemperature:STORAGE_OPTIONS,isHeatedMeatProducts:['該当なし','加熱食肉製品'],nutritionUnit:['100g','1個','1包','1袋']};
  if(key==='storageTemperature'&&draft.data[key]&&!options[key].includes(draft.data[key]))options[key]=[draft.data[key],...options[key]];
  if(key==='nutritionUnit'&&draft.data[key]&&!options[key].includes(draft.data[key]))options[key]=[draft.data[key],...options[key]];
  return {label:key==='template'?'ラベルの種類':key==='copies'?'枚数':key==='deadlineDate'?(draft.data.deadlineLabel||'期限日'):metadata?.label||key,
    type:options[key]?'select':['receivingDate','deadlineDate'].includes(key)?'date':key==='copies'?'number':'text',
    required:key==='copies'||key==='template'||key==='receivingDate'||!!metadata?.required,order:0,options:options[key],selectValueLabel:key==='template'};
}
export function labelGroups(draft:LabelDraft){
  const fields=labelFields(draft);
  const groups=[
    {id:'basic',label:'基本',keys:['productName','productQuantity','deadlineDate','deadlineLabel','storageTemperature']},
    {id:'individual',label:'個体',keys:['individualNumber','species','sex','captureLocation','receivingDate']},
    {id:'ingredients',label:'原材料',keys:['productIngredient','attentionText','isHeatedMeatProducts']},
    {id:'nutrition',label:'栄養成分',keys:['nutritionUnit','caloriesQuantity','proteinQuantity','fatQuantity','carbohydratesQuantity','saltEquivalentQuantity']},
  ].map(group=>({...group,keys:group.keys.filter(key=>fields.includes(key as PlaceholderName))})).filter(group=>group.keys.length);
  // No silent omissions when the upstream template acquires another editable placeholder.
  const assigned=new Set(groups.flatMap(group=>group.keys));const remainder=fields.filter(key=>!assigned.has(key));
  if(remainder.length)groups.push({id:'other',label:'その他',keys:remainder});
  return groups;
}
