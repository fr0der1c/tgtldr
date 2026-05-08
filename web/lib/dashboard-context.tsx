"use client";

import { createContext, PropsWithChildren, useContext, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { Bootstrap, Chat } from "@/lib/types";
import { onBootstrapRefresh } from "@/lib/bootstrap-sync";
import { useI18n } from "@/lib/i18n";

type DashboardContextValue = {
	bootstrap: Bootstrap | null;
	chats: Chat[];
	chatsReady: boolean;
	reloadChats: () => Promise<void>;
};

const DashboardContext = createContext<DashboardContextValue>({
	bootstrap: null,
	chats: [],
	chatsReady: false,
	reloadChats: async () => {},
});

export function DashboardProvider({ children }: PropsWithChildren) {
	const router = useRouter();
	const { setLanguage } = useI18n();
	const [bootstrap, setBootstrap] = useState<Bootstrap | null>(null);
	const [chats, setChats] = useState<Chat[]>([]);
	const [chatsReady, setChatsReady] = useState(false);

	useEffect(() => {
		function refreshBootstrap() {
			void api
				.bootstrap()
				.then((data) => {
					setBootstrap(data);
					setLanguage(data.language);
					if (data.passwordConfigured && !data.authenticated) {
						router.replace("/login");
					}
				})
				.catch(() => null);
		}

		refreshBootstrap();
		return onBootstrapRefresh(refreshBootstrap);
	}, [router, setLanguage]);

	useEffect(() => {
		void reloadChats();
	}, []);

	async function reloadChats() {
		try {
			const data = await api.listChats();
			setChats(data);
		} catch {
			// best-effort; individual panels surface errors as needed
		} finally {
			setChatsReady(true);
		}
	}

	return (
		<DashboardContext.Provider value={{ bootstrap, chats, chatsReady, reloadChats }}>
			{children}
		</DashboardContext.Provider>
	);
}

export function useDashboard() {
	return useContext(DashboardContext);
}
