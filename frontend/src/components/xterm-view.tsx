import {useEffect, useRef} from 'react'
import {FitAddon} from '@xterm/addon-fit'
import {Terminal} from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'

const ENTER = '\r'
const BACKSPACE = '\x7f'

export function XtermView({banner}: {banner: string[]}) {
    const containerRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        const container = containerRef.current
        if (!container) return

        const term = new Terminal({
            convertEol: true,
            fontSize: 13,
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
            cursorBlink: true,
            theme: {
                background: '#09090b',
                foreground: '#e4e4e7',
                cursor: '#e4e4e7',
            },
        })
        const fitAddon = new FitAddon()
        term.loadAddon(fitAddon)
        term.open(container)
        fitAddon.fit()

        banner.forEach((line) => term.writeln(line))
        term.write('$ ')

        let line = ''
        const dataListener = term.onData((data) => {
            if (data === ENTER) {
                term.write('\r\n')
                if (line.trim().length > 0) {
                    term.writeln(`mssh: ${line}: no active SSH connection`)
                }
                line = ''
                term.write('$ ')
            } else if (data === BACKSPACE) {
                if (line.length > 0) {
                    line = line.slice(0, -1)
                    term.write('\b \b')
                }
            } else if (data >= ' ') {
                line += data
                term.write(data)
            }
        })

        const resizeObserver = new ResizeObserver(() => fitAddon.fit())
        resizeObserver.observe(container)

        return () => {
            dataListener.dispose()
            resizeObserver.disconnect()
            term.dispose()
        }
    }, [banner])

    return <div ref={containerRef} className="h-full w-full"/>
}
