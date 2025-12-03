package colorspace

func RCTForward(r, g, b int32) (y, cb, cr int32) {
    y = (r + 2*g + b) >> 2
    cb = b - g
    cr = r - g
    return
}

func RCTInverse(y, cb, cr int32) (r, g, b int32) {
    g = y - ((cb + cr) >> 2)
    r = cr + g
    b = cb + g
    return
}

func ApplyRCTToComponents(r, g, b []int32) (y, cb, cr []int32) {
    n := len(r)
    y = make([]int32, n)
    cb = make([]int32, n)
    cr = make([]int32, n)
    for i := 0; i < n; i++ {
        y[i], cb[i], cr[i] = RCTForward(r[i], g[i], b[i])
    }
    return
}

func ApplyInverseRCTToComponents(y, cb, cr []int32) (r, g, b []int32) {
    n := len(y)
    r = make([]int32, n)
    g = make([]int32, n)
    b = make([]int32, n)
    for i := 0; i < n; i++ {
        r[i], g[i], b[i] = RCTInverse(y[i], cb[i], cr[i])
    }
    return
}
