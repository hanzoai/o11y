import {
	Dispatch,
	RefObject,
	SetStateAction,
	useEffect,
	useRef,
	useState,
} from 'react';
import { PartialOptions } from 'overlayscrollbars';
import { useOverlayScrollbars } from 'overlayscrollbars-react';

const useInitializeOverlayScrollbar = (
	options: PartialOptions,
): {
	setScroller: Dispatch<SetStateAction<HTMLElement | null>>;
	rootRef: RefObject<HTMLDivElement | null>;
} => {
	const rootRef = useRef<HTMLDivElement>(null);
	const [scroller, setScroller] = useState<HTMLElement | null>(null);
	const [initialize, osInstance] = useOverlayScrollbars({
		defer: true,
		options,
	});

	useEffect(() => {
		const { current: root } = rootRef;

		if (scroller && root) {
			initialize({
				target: root,
				elements: {
					viewport: scroller,
				},
			});
		}

		return (): void => osInstance()?.destroy();
	}, [scroller, initialize, osInstance]);

	return { setScroller, rootRef };
};

export default useInitializeOverlayScrollbar;
