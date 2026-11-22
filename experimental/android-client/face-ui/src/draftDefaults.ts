import type {FormDefinition} from './types/forms';
import {initializeFormValues} from './lib/forms/formRuntime';
import {missingValue} from './lib/voice-guidance/stationUi';

export function draftValues(form:FormDefinition,overrides:Record<string,unknown>) {
  return initializeFormValues(form,overrides,false);
}
/** Defaults are displayed as entered, but remain distinguishable from operator-confirmed facts. */
export function defaultFieldKeys(form:FormDefinition,overrides:Record<string,unknown>){
  const initial=initializeFormValues(form,undefined,false);
  return Object.keys(initial).filter(key=>!Object.hasOwn(overrides,key)&&!missingValue(initial[key]));
}
