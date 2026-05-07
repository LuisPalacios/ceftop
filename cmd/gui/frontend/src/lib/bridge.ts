// Bridge — typed wrapper over the auto-generated Wails Go bindings.
// In dev (`wails dev`) and built mode these call into the Go App; in a plain
// vite dev server the imports resolve to undefined and the calls reject.

import {
	GetConfig,
	GetPrivateIcons,
	SetTargetApp,
	SetTickInterval,
	OpenConfigInEditor,
	Snapshot,
	KillProcess,
	DiscoverApps,
	GetAppVersion,
	WindowSetSize,
	WindowGetSize,
	Log,
} from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

import type {
	ConfigState,
	DiscoveredApp,
	ProcessSnapshot,
	KillResult,
} from "./types";

export const bridge = {
	getConfig: (): Promise<ConfigState> => GetConfig() as unknown as Promise<ConfigState>,
	getPrivateIcons: (): Promise<Record<string, string>> =>
		GetPrivateIcons() as unknown as Promise<Record<string, string>>,
	setTargetApp: (name: string): Promise<void> => SetTargetApp(name),
	setTickInterval: (seconds: number): Promise<void> => SetTickInterval(seconds),
	openConfigInEditor: (): Promise<void> => OpenConfigInEditor(),
	snapshot: (): Promise<ProcessSnapshot> => Snapshot() as unknown as Promise<ProcessSnapshot>,
	killProcess: (pid: number): Promise<KillResult> => KillProcess(pid) as unknown as Promise<KillResult>,
	discoverApps: (): Promise<DiscoveredApp[]> => DiscoverApps() as unknown as Promise<DiscoveredApp[]>,
	getAppVersion: (): Promise<string> => GetAppVersion(),
	windowSetSize: (width: number, height: number): Promise<void> => WindowSetSize(width, height),
	windowGetSize: (): Promise<[number, number]> => WindowGetSize() as unknown as Promise<[number, number]>,
	log: (msg: string): Promise<void> => Log(msg),

	onSnapshot: (handler: (snap: ProcessSnapshot) => void): (() => void) => {
		EventsOn("snapshot", handler as (...args: unknown[]) => void);
		return () => EventsOff("snapshot");
	},
	onSnapshotError: (handler: (err: string) => void): (() => void) => {
		EventsOn("snapshot:error", handler as (...args: unknown[]) => void);
		return () => EventsOff("snapshot:error");
	},
	onDiscovery: (handler: (apps: DiscoveredApp[]) => void): (() => void) => {
		EventsOn("discovery", handler as (...args: unknown[]) => void);
		return () => EventsOff("discovery");
	},
	onDiscoveryError: (handler: (err: string) => void): (() => void) => {
		EventsOn("discovery:error", handler as (...args: unknown[]) => void);
		return () => EventsOff("discovery:error");
	},
};
