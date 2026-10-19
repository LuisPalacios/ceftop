// Bridge — typed wrapper over the auto-generated Wails Go bindings.
// In dev (`wails dev`) and built mode these call into the Go App; in a plain
// vite dev server the imports resolve to undefined and the calls reject.

import {
	GetConfig,
	DescribeApp,
	SetTargetApp,
	SetTickInterval,
	OpenConfigInEditor,
	OpenCatalogInEditor,
	RenameHost,
	KillProcess,
	GetAppVersion,
	FitWindowToScreen,
	Log,
	FrontendReady,
} from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

import type {
	AppInfo,
	ConfigState,
	DiscoveredApp,
	ProcessSnapshot,
	KillResult,
} from "./types";

export const bridge = {
	getConfig: (): Promise<ConfigState> => GetConfig() as unknown as Promise<ConfigState>,
	describeApp: (name: string): Promise<AppInfo> => DescribeApp(name) as unknown as Promise<AppInfo>,
	setTargetApp: (name: string): Promise<void> => SetTargetApp(name),
	setTickInterval: (seconds: number): Promise<void> => SetTickInterval(seconds),
	openConfigInEditor: (): Promise<void> => OpenConfigInEditor(),
	// Creates apps.json pre-filled with the hosts on screen when missing.
	openCatalogInEditor: (): Promise<void> => OpenCatalogInEditor(),
	// Names a host app inside a runtime's tree; "" restores the default.
	renameHost: (runtime: string, host: string, label: string): Promise<void> =>
		RenameHost(runtime, host, label),
	killProcess: (pid: number): Promise<KillResult> => KillProcess(pid) as unknown as Promise<KillResult>,
	getAppVersion: (): Promise<string> => GetAppVersion(),
	// Caps the fixed-size window to the current monitor's work area.
	fitWindowToScreen: (screenW: number, screenH: number): Promise<void> =>
		FitWindowToScreen(screenW, screenH),
	log: (msg: string): Promise<void> => Log(msg),
	// Signals that event subscriptions are wired; the backend answers with
	// an immediate snapshot + discovery emit instead of the next tick.
	frontendReady: (): Promise<void> => FrontendReady(),

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
