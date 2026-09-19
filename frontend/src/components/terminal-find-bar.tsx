import {useEffect, useRef, useState} from 'react'
import {ChevronDown, ChevronUp, X} from 'lucide-react'

import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {
    clearTerminalSearch,
    findInTerminal,
    onTerminalSearchResults,
} from '@/lib/terminal-session'

/**
 * The find bar one terminal
 *
 * It owns the search term and nothing else. The matches, the highlights and the
 * current position all live in the terminal, outside React - this components
 * only asks questions and displays the answers
 * */
export function TerminalFindBar({
    connectionId,
    onClose,
}: {
    connectionId: string,
    onClose: () => void
}) {
    const [term, setTerm] = useState('')
    const [resultIndex, setResultIndex] = useState(-1)
    const [resultCount, setResultCount] = useState(0)
    const inputRef = useRef<HTMLInputElement>(null)
    
    useEffect(() => {
        inputRef.current?.focus()
    }, [])

    useEffect(() => {
        return onTerminalSearchResults(connectionId, (results) => {
            setResultIndex(results.resultIndex)
            setResultCount(results.resultCount)
        })
    }, [connectionId])

    // Closing the bar must not leave highlights behind on the terminal
    useEffect(() => {
        return () => clearTerminalSearch(connectionId)
    }, [connectionId])

    
    function jumpTo(direction: 'next' | 'previous') {
        findInTerminal(connectionId, {term, direction, incremental: false})
    }

    // resultIndex is -1 when there are more matches than the addon will
    // highlight, so there is a real answer but no "current one" to number.
    let summary = ''
    if (term !== '') {
        if (resultCount === 0) {
            summary = 'No matches'
        } else if (resultIndex < 0) {
            summary = `${resultCount} matches`
        } else {
            summary = `${resultIndex + 1} of ${resultCount}`
        }
    }

    return (
        <div className="flex items-center gap-2 rounded-lg border bg-card p-2">
            <Input
                ref={inputRef}
                value={term}
                placeholder="Find in terminal"
                className="flex-1"
                onChange={(event) => {
                    // nextTerm, not term: state set in this handler is not
                    // readable until the next render.
                    const nextTerm = event.target.value
                    setTerm(nextTerm)
                    findInTerminal(connectionId, {
                        term: nextTerm,
                        direction: 'next',
                        incremental: true,
                    })
                }}
                onKeyDown={(event) => {
                    if (event.key === 'Escape') {
                        event.preventDefault()
                        onClose()
                        return
                    }
                    if (event.key === 'Enter') {
                        event.preventDefault()
                        jumpTo(event.shiftKey ? 'previous' : 'next')
                    }
                }}
            />

            <span className="w-24 shrink-0 text-right text-xs text-muted-foreground">
                {summary}
            </span>

            <Button
                variant="ghost"
                size="icon"
                onClick={() => jumpTo('previous')}
                title="Previous match (⇧⏎)"
            >
                <ChevronUp/>
            </Button>
            <Button
                variant="ghost"
                size="icon"
                onClick={() => jumpTo('next')}
                title="Next match (⏎)"
            >
                <ChevronDown/>
            </Button>
            <Button variant="ghost" size="icon" onClick={onClose} title="Close (Esc)">
                <X/>
            </Button>
        </div>
    )
}
