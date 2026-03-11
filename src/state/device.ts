export interface DeviceState {
  scaleConnected: boolean;
  scalePort: string;
  printerConnected: boolean;
  currentWeight: number | null;
  currentUnit: string;
  lastError: string | null;
}

export class DeviceStateManager {
  private state: DeviceState = {
    scaleConnected: false,
    scalePort: '',
    printerConnected: false,
    currentWeight: null,
    currentUnit: 'kg',
    lastError: null,
  };

  private listeners: Array<(state: DeviceState) => void> = [];

  get(): Readonly<DeviceState> {
    return { ...this.state };
  }

  update(partial: Partial<DeviceState>): void {
    this.state = { ...this.state, ...partial };
    for (const listener of this.listeners) {
      listener(this.state);
    }
  }

  onUpdate(listener: (state: DeviceState) => void): () => void {
    this.listeners.push(listener);
    return () => {
      this.listeners = this.listeners.filter((l) => l !== listener);
    };
  }
}
