import {validDate,validQuantity} from './lib/voice-guidance/labelEditor';
export type CatalogItem=Record<string,unknown>&{id:string;individualNumber?:string;name?:string};
export function individualData(row:CatalogItem):Record<string,string>{
  const out:Record<string,string>={individualNumber:'',species:'',sex:'',captureLocation:'',receivingDate:''};
  for(const key of Object.keys(out)){const value=row[key];if(typeof value==='string')out[key]=value;}
  const date=String(row.receivingDate||row.timeToFactory||'').slice(0,10);out.receivingDate=validDate(date)?date:'';
  return out;
}
export function productData(row:CatalogItem):Record<string,string>{
  const out:Record<string,string>={productName:String(row.name||''),productQuantity:'',productIngredient:'',nutritionUnit:'',caloriesQuantity:'',proteinQuantity:'',fatQuantity:'',carbohydratesQuantity:'',saltEquivalentQuantity:'',attentionText:'',isHeatedMeatProducts:''};
  for(const key of Object.keys(out))if(key!=='productName'&&(typeof row[key]==='string'||typeof row[key]==='number'))out[key]=String(row[key]);
  out.productIngredient=String(row.productIngredient||row.ingredient||'');
  const quantity=String(row.defaultContentQuantity||'');out.productQuantity=validQuantity(quantity)?quantity:validQuantity(quantity+String(row.unit||''))?quantity+row.unit:'';
  out.isHeatedMeatProducts=row.isHeatedMeatProducts===true?'加熱食肉製品':row.isHeatedMeatProducts===false?'該当なし':out.isHeatedMeatProducts;
  if(Number.isInteger(row.expirationDays)&&Number(row.expirationDays)>0&&Number(row.expirationDays)<3650){const d=new Date();d.setDate(d.getDate()+Number(row.expirationDays));out.deadlineDate=`${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`;}else out.deadlineDate='';
  return out;
}
export function scanQuery(value:string):{query:string}|{id:string;trace:boolean}{
  const text=value.trim();if(/^[A-Za-z0-9-]{1,15}$/.test(text))return {query:text};
  let url:URL;try{url=new URL(text);}catch{throw new Error('個体番号またはラクシカのコードを読み取ってください。');}
  if(url.protocol!=='https:'||!['rakusika.com','www.rakusika.com'].includes(url.hostname)||url.port||url.username||url.password)throw new Error('ラクシカのコードを読み取ってください。');
  const match=url.pathname.match(/^\/(t|trace)\/([A-Za-z0-9_-]{1,128})(?:\/[^/]+)?\/?$/);if(!match)throw new Error('このコードには対応していません。個体番号で検索してください。');return {id:match[2],trace:match[1]==='t'};
}
export function swipeDirection(dx:number,dy:number){return Math.abs(dx)>=48&&Math.abs(dx)>Math.abs(dy)*1.5?(dx<0?1:-1):0;}

export function productFamilyMatches(row:CatalogItem,family?:string){
 if(!family)return true;if(family==='meat')return row.isTraceable===true;
 if(row.isTraceable===true)return false;
 const type=String(row.productType||'').trim().toLowerCase();
 return family==='pet'?['pet','ペットフード','ペット向け製品'].includes(type):['','processed','加工品'].includes(type);
}
